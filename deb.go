package packit

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
)

const (
	debVersion     = "2.0\n"
	debDataTar     = "data.tar.gz"
	debControlTar  = "control.tar.gz"
	debBinaryFile  = "debian-binary"
	debControlFile = "control"
	debSumFile     = "md5sums"
	debConfFile    = "conffiles"
	debPreinst     = "preinst"
	debPostinst    = "postinst"
	debPrerem      = "prerm"
	debPostrem     = "postrm"
)

const debChangelog = `{{range .Changes}}  {{$.Control.Package}} ({{$.Control.Version}}) unstable; urgency=low

{{range .Changes}}   * {{.}}
{{end}}
  -- {{.Maintainer.Name}} <{{.Maintainer.Email}}> {{strftime .When}}
{{end}}`

const debControl = `
Package: {{.Package}}
Version: {{.Version}}
License: {{.License}}
Section: {{.Section}}
Priority: {{.Priority}}
Architecture: {{arch .Arch}}
Vendor: {{.Vendor}}
Maintainer: {{.Name}} <{{.Email}}>
Homepage: {{.Home}}
{{if .Depends }}Depends: {{join .Depends ", "}}{{end}}
{{if .Suggests }}Suggests: {{join .Suggests ", "}}{{end}}
Installed-Size: {{.Size}}
Build-Using: {{.Compiler}}
Description: {{.Summary}}
{{indent .Desc}}
`

type DEB struct {
	*Makefile
}

func (d *DEB) PackageName() string {
	return d.Control.PackageName() + ".deb"
}

func (d *DEB) Build(w io.Writer) error {
	aw, err := ar.NewWriter(w)
	if err != nil {
		return err
	}
	defer aw.Close()

	if err := d.writeDebian(aw); err != nil {
		return err
	}
	var data, control bytes.Buffer
	if err := d.writeData(&data); err != nil {
		return err
	}
	if err := d.writeControl(&control); err != nil {
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
			ModTime:  time.Now().Truncate(time.Minute),
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
	return nil
}

func (d *DEB) writeData(w io.Writer) error {
	wt := tar.NewWriter(w)

	done, err := d.writeChangelog(wt)
	if err != nil {
		return err
	}
	digest := md5.New()
	sort.Slice(d.Files, func(i, j int) bool { return d.Files[i].String() < d.Files[j].String() })
	for _, i := range d.Files {
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
		if done, err = tarIntermediateDirectories(wt, i.String(), done); err != nil {
			return err
		}
		h := tar.Header{
			Name:     i.String(),
			Mode:     i.Mode(),
			Size:     size,
			ModTime:  time.Now().Truncate(time.Minute),
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
		i.Sum = fmt.Sprintf("%x", digest.Sum(nil))

		f.Close()
		digest.Reset()
	}
	return wt.Close()
}

func (d *DEB) writeControl(w io.Writer) error {
	var ds, cs []string
	for _, f := range d.Files {
		if f.Conf {
			n := f.String()
			if !strings.HasPrefix(n, "/") {
				n = "/" + n
			}
			cs = append(cs, n)
		}
		ds = append(ds, fmt.Sprintf("%s %s", f.Sum, f.String()))
		d.Control.Size += f.Size
	}
	wt := tar.NewWriter(w)
	if err := d.writeControlFile(wt); err != nil {
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
			ModTime:  time.Now().Truncate(time.Minute),
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

func (d *DEB) writeControlFile(w *tar.Writer) error {
	fs := template.FuncMap{
		"join":   strings.Join,
		"arch":   debArch,
		"indent": debIndent,
	}
	var body bytes.Buffer
	t, err := template.New("control").Funcs(fs).Parse(strings.TrimSpace(debControl) + "\n")
	if err != nil {
		return err
	}
	d.Control.Size = d.Control.Size >> 10
	if err := t.Execute(cleanBlank(&body), *d.Control); err != nil {
		return err
	}
	h := tar.Header{
		Name:     debControlFile,
		ModTime:  time.Now().Truncate(time.Minute),
		Uid:      0,
		Gid:      0,
		Mode:     0644,
		Size:     int64(body.Len()),
		Typeflag: tar.TypeReg,
	}
	if err := w.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(w, &body)
	return err
}

func (d *DEB) writeChangelog(w *tar.Writer) (map[string]struct{}, error) {
	done := make(map[string]struct{})
	if len(d.Changes) == 0 {
		return done, nil
	}
	for _, g := range d.Changes {
		if g.Maintainer == nil {
			g.Maintainer = d.Control.Maintainer
		}
	}
	var body bytes.Buffer
	z, _ := gzip.NewWriterLevel(&body, gzip.BestCompression)
	fs := template.FuncMap{
		"strftime": func(t time.Time) string { return t.Format("Mon, 02 Jan 2006 15:04:05 -0700") },
	}
	t, err := template.New("changelog").Funcs(fs).Parse(debChangelog)
	if err != nil {
		return nil, err
	}
	if err := t.Execute(z, d); err != nil {
		return nil, err
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	name := filepath.Join("usr/share/doc", d.Control.Package, "changelog.gz")
	if done, err = tarIntermediateDirectories(w, name, done); err != nil {
		return nil, err
	}
	h := tar.Header{
		Name:     name,
		Mode:     0644,
		Uid:      0,
		Gid:      0,
		Size:     int64(body.Len()),
		ModTime:  time.Now().Truncate(time.Minute),
		Typeflag: tar.TypeReg,
	}
	if err := w.WriteHeader(&h); err != nil {
		return nil, err
	}
	digest := md5.New()
	if n, err := io.Copy(io.MultiWriter(w, digest), &body); err != nil {
		return nil, err
	} else {
		f := File{
			Name: h.Name,
			Size: int64(n),
			Sum:  fmt.Sprintf("%x", digest.Sum(nil)),
		}
		d.Files = append(d.Files, &f)
	}
	return done, nil
}

func (d *DEB) writeDebian(a *ar.Writer) error {
	h := tape.Header{
		Filename: debBinaryFile,
		Uid:      0,
		Gid:      0,
		ModTime:  time.Now().Truncate(time.Minute),
		Mode:     0644,
		Length:   int64(len(debVersion)),
	}
	if err := a.WriteHeader(&h); err != nil {
		return err
	}
	_, err := io.WriteString(a, debVersion)
	return err
}

type blank struct {
	io.Writer
	last byte
}

func cleanBlank(w io.Writer) io.Writer {
	return &blank{Writer: w}
}

func (b *blank) Write(bs []byte) (int, error) {
	var (
		xs     []byte
		offset int
	)
	if b.last != 0 {
		xs = make([]byte, len(bs)+1)
		xs[0], offset = b.last, 1
	} else {
		xs = make([]byte, len(bs))
	}
	copy(xs[offset:], bs)

	xs = bytes.Replace(xs, []byte{0x0a, 0x0a}, []byte{0x0a}, -1)
	b.last = bs[len(bs)-1]
	_, err := b.Writer.Write(xs[offset:])
	return len(bs), err
}

func debArch(a uint8) string {
	switch a {
	case 32:
		return "i386"
	case 64:
		return "amd64"
	default:
		return "all"
	}
}

func debIndent(dsc string) string {
	var body bytes.Buffer
	s := bufio.NewScanner(strings.NewReader(dsc))
	for s.Scan() {
		x := s.Text()
		if x == "" {
			io.WriteString(&body, " .\n")
		} else {
			io.WriteString(&body, " "+x+"\n")
		}
	}
	return body.String()
}

func tarIntermediateDirectories(w *tar.Writer, n string, done map[string]struct{}) (map[string]struct{}, error) {
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
			ModTime:  time.Now().Truncate(time.Minute),
			Mode:     0755,
			Gid:      0,
			Uid:      0,
			Typeflag: tar.TypeDir,
		}
		if err := w.WriteHeader(&h); err != nil {
			return nil, err
		}
	}
	return done, nil
}
