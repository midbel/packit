package apk

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/md5"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/packit"
	"github.com/midbel/packit/text"
	"github.com/midbel/tape/tar"
)

const (
	apkControlFile   = "control.tar.gz"
	apkDataFile      = "data.tar.gz"
	apkPackageFile   = ".PKGINFO"
	apkSignatureFile = ".SIGN"
)

func Extract(file, dir string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := getData(bufio.NewReader(f))
	if err != nil {
		return err
	}
	if c, ok := r.(io.Closer); ok {
		defer c.Close()
	}
	rt := tar.NewReader(r)
	for {
		h, err := rt.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if h.Type != tar.TypeReg {
			continue
		}
		r := io.LimitReader(rt, h.Size)
		if err := extractFile(r, filepath.Join(dir, h.Name), h.Perm); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(r io.Reader, file string, perm int64) error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	w, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, os.FileMode(perm))
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = io.Copy(w, r)
	return err
}

func Info(file string) (packit.Metadata, error) {
	f, err := os.Open(file)
	if err != nil {
		return packit.Metadata{}, err
	}
	defer f.Close()

	r, err := getFile(bufio.NewReader(f), apkControlFile, apkPackageFile)
	if err != nil {
		return packit.Metadata{}, err
	}
	return ParseControl(r)
}

func Verify(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := getData(bufio.NewReader(f))
	if err != nil {
		return err
	}
	if c, ok := r.(io.Closer); ok {
		defer c.Close()
	}
	var (
		md = md5.New()
		rt = tar.NewReader(io.TeeReader(r, md))
	)
	for {
		h, err := rt.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if h.Type != tar.TypeReg {
			io.Copy(io.Discard, rt)
			continue
		}
		sh1 := sha1.New()
		if _, err := io.CopyN(sh1, rt, h.Size); err != nil {
			return err
		}
		var (
			want = h.PaxHeaders["APK-TOOLS.checksum.SHA1"]
			got  = fmt.Sprintf("%x", sh1.Sum(nil))
		)
		if got != want && want != "" {
			return fmt.Errorf("%s: checksum mismatched!", h.Name)
		}
	}
	return nil
}

func List(file string) ([]packit.Resource, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := getData(bufio.NewReader(f))
	if err != nil {
		return nil, err
	}
	if c, ok := r.(io.Closer); ok {
		defer c.Close()
	}
	var (
		rt   = tar.NewReader(r)
		list []packit.Resource
	)
	for {
		h, err := rt.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if _, err := io.Copy(io.Discard, io.LimitReader(rt, h.Size)); err != nil {
			return nil, err
		}
		r := packit.Resource{
			File:    h.Name,
			Perm:    int(h.Perm),
			Size:    h.Size,
			ModTime: h.ModTime,
		}
		list = append(list, r)
	}
	return list, nil
}

func Build(dir string, meta packit.Metadata) error {
	w, err := os.Create(filepath.Join(dir, getPackageName(meta)))
	if err != nil {
		return err
	}
	defer w.Close()
	return build(w, meta)
}

func build(w io.Writer, meta packit.Metadata) error {
	var (
		data bytes.Buffer
		hash = sha256.New()
	)
	if err := writeData(io.MultiWriter(&data, hash), meta); err != nil {
		return err
	}
	meta.DataHash = fmt.Sprintf("%x", hash.Sum(nil))
	if err := writeControl(w, meta); err != nil {
		return err
	}
	_, err := io.Copy(w, &data)
	return err
}

func writeControl(w io.Writer, meta packit.Metadata) error {
	var (
		tmp    bytes.Buffer
		buf, _ = gzip.NewWriterLevel(&tmp, gzip.BestCompression)
		tw     = tar.NewWriter(buf)
	)
	buf.Header.Name = apkControlFile
	if err := appendControlFile(tw, meta); err != nil {
		return err
	}
	tw.Flush()
	buf.Close()

	_, err := io.Copy(w, &tmp)
	return err
}

func writeData(w io.Writer, meta packit.Metadata) error {
	sort.Slice(meta.Resources, func(i, j int) bool {
		return meta.Resources[i].Path() < meta.Resources[j].Path()
	})
	var (
		tmp    bytes.Buffer
		buf, _ = gzip.NewWriterLevel(&tmp, gzip.BestCompression)
		tw     = tar.NewWriter(buf)
		dirs   = make(map[string]struct{})
	)
	buf.Header.Name = apkDataFile
	for _, r := range meta.Resources {
		if err := appendResource(tw, r, dirs); err != nil {
			return err
		}
	}
	tw.Close()
	buf.Close()
	_, err := io.Copy(w, &tmp)
	return err
}

func appendResource(tw *tar.Writer, res packit.Resource, dirs map[string]struct{}) error {
	var (
		dir = filepath.Dir(res.Path())
		tmp string
	)
	for _, d := range strings.Split(dir, "/") {
		tmp = filepath.Join(tmp, d)
		if _, ok := dirs[tmp]; ok {
			continue
		}
		dirs[tmp] = struct{}{}
		h := getTarHeaderDir(tmp, res.ModTime)
		if err := tw.WriteHeader(&h); err != nil {
			return err
		}
	}
	r, err := os.Open(res.File)
	if err != nil {
		return err
	}
	defer r.Close()

	var (
		buf bytes.Buffer
		w   io.Writer = &buf
	)
	if res.Compress {
		w, _ = gzip.NewWriterLevel(w, gzip.BestCompression)
	}
	if res.Size, err = io.Copy(w, r); err != nil {
		return err
	}
	if c, ok := w.(io.Closer); ok {
		c.Close()
	}
	h := getTarHeaderFile(res.Path(), res.Perm, buf.Len(), res.ModTime)
	h.PaxHeaders["APK-TOOLS.checksum.SHA1"] = res.Digest
	if err := tw.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(tw, &buf)
	return err
}

//go:embed control.tpl
var controlfile string

var fmap = template.FuncMap{}

func appendControlFile(tw *tar.Writer, meta packit.Metadata) error {
	tpl, err := template.New("control").Funcs(fmap).Parse(controlfile)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := text.Execute(tpl, &buf, meta); err != nil {
		return err
	}

	h := getTarHeaderFile(apkPackageFile, 0644, buf.Len(), meta.Date)
	if err := tw.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(tw, &buf)
	return err
}

func getPackageName(meta packit.Metadata) string {
	return fmt.Sprintf("%s-%s.%s", meta.Package, meta.Version, packit.APK)
}

func getTarHeaderFile(file string, perm, size int, when time.Time) tar.Header {
	return tar.Header{
		Name:    file,
		Perm:    int64(perm),
		Size:    int64(size),
		ModTime: when,
		Gid:     0,
		Uid:     0,
		Type:    tar.TypeReg,
		PaxHeaders: map[string]string{
			"atime": "0",
			"mtime": "0",
		},
	}
}

func getTarHeaderDir(file string, when time.Time) tar.Header {
	return tar.Header{
		Name:    file,
		Perm:    0755,
		ModTime: when,
		Gid:     0,
		Uid:     0,
		Type:    tar.TypeDir,
		PaxHeaders: map[string]string{
			"atime": "0",
			"mtime": "0",
		},
	}
}

func getFile(r io.Reader, zone, file string) (io.Reader, error) {
	var ret io.Reader
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	for {
		zr.Multistream(false)
		if zr.Name == zone {
			ret = zr
			break
		}
		if _, err := io.Copy(io.Discard, zr); err != nil {
			return nil, err
		}
		if err := zr.Reset(r); err != nil {
			return nil, err
		}
	}
	if file == "" {
		return ret, nil
	}
	rt := tar.NewReader(ret)
	for {
		h, err := rt.Next()
		if err != nil {
			return nil, err
		}
		if h.Name == file {
			return io.LimitReader(rt, h.Size), nil
		}
		if _, err := io.CopyN(io.Discard, rt, h.Size); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%s: file not found in %s", file, zone)
}

func getData(r io.Reader) (io.Reader, error) {
	return getFile(r, apkDataFile, "")
}

func getControl(r io.Reader) (io.Reader, error) {
	return getFile(r, apkControlFile, "")
}
