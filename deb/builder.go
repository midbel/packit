package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/midbel/packit"
	"github.com/midbel/packit/deb/changelog"
	"github.com/midbel/packit/deb/control"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
)

type builder struct {
	when time.Time

	control *packit.Control
	files   []*packit.File
	changes []*packit.Change
}

func (b *builder) PackageName() string {
	if b.control == nil {
		return "packit.deb"
	}
	return b.control.PackageName() + ".deb"
}

func (b *builder) Build(w io.Writer) error {
	aw, err := ar.NewWriter(w)
	if err != nil {
		return err
	}
	if err := b.writeDebian(aw); err != nil {
		return err
	}
	var data, control bytes.Buffer
	if err := b.writeData(&data); err != nil {
		return err
	}
	if err := b.writeControl(&control); err != nil {
		return err
	}
	ts := []struct {
		File   string
		Buffer bytes.Buffer
	}{
		{File: debControlTar, Buffer: control},
		{File: debDataTar, Buffer: data},
	}
	for _, t := range ts {
		var body bytes.Buffer
		z := gzip.NewWriter(&body)
		if _, err := io.Copy(z, &t.Buffer); err != nil {
			return err
		}
		if err := z.Close(); err != nil {
			return err
		}
		h := tape.Header{
			Filename: t.File,
			Uid:      0,
			Gid:      0,
			ModTime:  b.when,
			Mode:     0644,
			Length:   int64(body.Len()),
		}
		if err := aw.WriteHeader(&h); err != nil {
			return err
		}
		if _, err := io.Copy(aw, &body); err != nil {
			return err
		}
	}
	return aw.Close()
}

func (b *builder) writeData(w io.Writer) error {
	wt := tar.NewWriter(w)
	done := make(map[string]struct{})

	if err := b.writeChangelog(wt, done); err != nil {
		return err
	}
	digest := md5.New()
	sort.Slice(b.files, func(i, j int) bool { return b.files[i].String() < b.files[j].String() })
	for _, i := range b.files {
		if i.Src == "" && i.Dst == "" {
			continue
		}
		f, err := os.Open(i.Src)
		if err != nil {
			return err
		}
		var (
			r    io.Reader
			size int64
		)
		if i.Compress {
			var rs bytes.Buffer
			z, _ := gzip.NewWriterLevel(&rs, gzip.BestCompression)
			if _, err := io.Copy(z, f); err != nil {
				return err
			}
			if err := z.Close(); err != nil {
				return err
			}
			size, r = int64(rs.Len()), &rs
		} else {
			s, err := f.Stat()
			if err != nil {
				return err
			}
			size, r = s.Size(), f
		}
		if err := makeIntermediateDirectories(wt, i.String(), done); err != nil {
			return err
		}
		h := tar.Header{
			Name:     i.String(),
			Mode:     i.Mode(),
			Size:     size,
			ModTime:  b.when,
			Gid:      0,
			Uid:      0,
			Typeflag: tar.TypeReg,
		}
		if err := wt.WriteHeader(&h); err != nil {
			return err
		}
		if i.Size, err = io.Copy(wt, io.TeeReader(r, digest)); err != nil {
			return err
		}
		i.Sum = hex.EncodeToString(digest.Sum(nil))

		f.Close()
		digest.Reset()
	}
	return wt.Close()
}

func (b *builder) writeChangelog(w *tar.Writer, done map[string]struct{}) error {
	if len(b.changes) == 0 {
		return nil
	}
	for _, g := range b.changes {
		if g.Maintainer == nil {
			g.Maintainer = b.control.Maintainer
		}
	}
	var body bytes.Buffer
	if err := changelog.DumpCompressed(b.control.Package, b.changes, &body); err != nil {
		return err
	}
	name := filepath.Join("usr/share/doc", b.control.Package, debChangeFile)
	if err := makeIntermediateDirectories(w, name, done); err != nil {
		return err
	}
	h := tar.Header{
		Name:     name,
		Mode:     0644,
		Uid:      0,
		Gid:      0,
		Size:     int64(body.Len()),
		ModTime:  b.when,
		Typeflag: tar.TypeReg,
	}
	if err := w.WriteHeader(&h); err != nil {
		return err
	}
	digest := md5.New()
	if n, err := io.Copy(io.MultiWriter(w, digest), &body); err != nil {
		return err
	} else {
		f := packit.File{
			Name: h.Name,
			Size: int64(n),
			Sum:  hex.EncodeToString(digest.Sum(nil)),
		}
		b.files = append(b.files, &f)
	}
	return nil
}

func (b *builder) writeControl(w io.Writer) error {
	var ds, cs []string
	for _, f := range b.files {
		if f.Conf {
			n := f.String()
			if !strings.HasPrefix(n, "/") {
				n = "/" + n
			}
			cs = append(cs, n)
		}
		ds = append(ds, fmt.Sprintf("%s %s", f.Sum, f.String()))
		b.control.Size += f.Size
	}
	wt := tar.NewWriter(w)
	if err := b.writeControlFile(wt); err != nil {
		return err
	}
	fs := []struct {
		File string
		Data []string
	}{
		{File: debSumFile, Data: ds},
		{File: debConfFile, Data: cs},
	}
	for _, f := range fs {
		if len(f.Data) == 0 {
			continue
		}
		var body bytes.Buffer
		for _, d := range f.Data {
			io.WriteString(&body, d+"\n")
		}
		h := tar.Header{
			Name:     f.File,
			ModTime:  b.when,
			Uid:      0,
			Gid:      0,
			Mode:     0644,
			Size:     int64(body.Len()),
			Typeflag: tar.TypeReg,
		}
		if err := wt.WriteHeader(&h); err != nil {
			return err
		}
		if _, err := io.Copy(wt, &body); err != nil {
			return err
		}
	}
	return wt.Close()
}

func (b *builder) writeControlFile(w *tar.Writer) error {
	var body bytes.Buffer
	if err := control.Dump(b.control, &body); err != nil {
		return err
	}
	h := tar.Header{
		Name:     debControlFile,
		ModTime:  b.when,
		Uid:      0,
		Gid:      0,
		Mode:     0644,
		Size:     int64(body.Len()),
		Typeflag: tar.TypeReg,
	}
	if err := w.WriteHeader(&h); err != nil {
		return err
	}
	_, err := io.Copy(w, &body)
	return err
}

func (b *builder) writeDebian(w tape.Writer) error {
	h := tape.Header{
		Filename: debBinaryFile,
		Uid:      0,
		Gid:      0,
		Mode:     0644,
		Length:   int64(len(debVersion)),
		ModTime:  b.when,
	}
	if err := w.WriteHeader(&h); err != nil {
		return err
	}
	_, err := io.WriteString(w, debVersion)
	return err
}

func makeIntermediateDirectories(w *tar.Writer, n string, done map[string]struct{}) error {
	ds := strings.Split(filepath.Dir(n), "/")
	for i := 0; i < len(ds); i++ {
		n := ds[i]
		if i > 0 {
			n = filepath.Join(strings.Join(ds[:i], "/"), n)
		}
		if _, ok := done[n]; ok {
			continue
		}
		done[n] = struct{}{}
		h := tar.Header{
			Name:     n + "/",
			ModTime:  time.Now(),
			Mode:     0755,
			Gid:      0,
			Uid:      0,
			Typeflag: tar.TypeDir,
		}
		if err := w.WriteHeader(&h); err != nil {
			return err
		}
	}
	return nil
}
