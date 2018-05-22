package deb

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/mack"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
)

const (
	DebVersion       = "2.0\n"
	DebDataTar       = "data.tar.gz"
	DebControlTar    = "control.tar.gz"
	DebBinaryFile    = "debian-binary"
	DebControlFile   = "./control"
	DebMD5sumsFile   = "./md5sums"
	DebConffilesFile = "./conffiles"
)

const control = `
Package: {{.Package}}
Version: {{.Version}}
License: {{.License}}
Section: {{.Section}}
Priority: {{.Priority}}
Architecture: {{.Arch}}
Vendor: {{.Vendor}}
Maintainer: {{.Name}} <{{.Email}}>
Homepage: {{.Home}}
Pre-Depends: {{join .Depends ", "}}
Installed-Size: {{.Size}}
Build-Using: {{.Compiler}}
Description: {{.Summary}}
{{indent .Desc}}
`

type builder struct {
	inner   *ar.Writer
	modtime time.Time

	size      int64
	conffiles []string
	md5sums   []string

	control *tarball
	data    *tarball
}

func NewBuilder(w io.Writer) (mack.Builder, error) {
	n := time.Now()
	aw, err := ar.NewWriter(w)
	if err != nil {
		return nil, err
	}
	if err := writeDebianBinaryFile(aw, n); err != nil {
		return nil, err
	}
	wd := &builder{
		inner:   aw,
		modtime: n,
		control: newTarball(DebControlTar),
		data:    newTarball(DebDataTar),
	}
	return wd, nil
}

func (w *builder) Build(c mack.Control, files []*mack.File) error {
	for _, f := range files {
		if err := w.writeFile(f); err != nil {
			return err
		}
	}
	c.Size = int(w.size / 1024)
	if err := w.writeControl(c); err != nil {
		return err
	}
	return w.flush()
}

func (w *builder) writeControl(c mack.Control) error {
	body, err := prepareControl(c)
	if err != nil {
		return err
	}
	return w.control.WriteString(DebControlFile, body.String(), w.modtime)
}

func (w *builder) writeFile(f *mack.File) error {
	r, err := os.Open(f.Src)
	if err != nil {
		return err
	}
	defer r.Close()
	i, err := r.Stat()
	if err != nil {
		return err
	}
	w.size += i.Size()
	if f.Conf || mack.IsConfFile(f.Dst) {
		p := f.String()
		if s := string(os.PathSeparator); !strings.HasPrefix(p, s) {
			p = s + p
		}
		w.conffiles = append(w.conffiles, p)
	}
	sum, err := w.data.WriteFile(f, w.modtime)
	if err != nil {
		return err
	}
	if len(sum) == md5.Size {
		w.md5sums = append(w.md5sums, fmt.Sprintf("%x %s", sum, f.String()))
	}
	return nil
}

func (w *builder) flush() error {
	if len(w.md5sums) > 0 {
		body := new(bytes.Buffer)
		for _, s := range w.md5sums {
			io.WriteString(body, s+"\n")
		}
		if err := w.control.WriteString(DebMD5sumsFile, body.String(), w.modtime); err != nil {
			return err
		}
	}
	if len(w.conffiles) > 0 {
		body := new(bytes.Buffer)
		for _, s := range w.conffiles {
			io.WriteString(body, s+"\n")
		}
		if err := w.control.WriteString(DebConffilesFile, body.String(), w.modtime); err != nil {
			return err
		}
	}
	for _, t := range []*tarball{w.control, w.data} {
		if err := writeTarball(w.inner, t, w.modtime); err != nil {
			return err
		}
	}
	return w.inner.Close()
}

func writeTarball(a *ar.Writer, t *tarball, n time.Time) error {
	if err := t.Close(); err != nil {
		return err
	}
	h := tape.Header{
		Filename: t.Name,
		Uid:      0,
		Gid:      0,
		ModTime:  n,
		Mode:     0644,
		Length:   int64(t.body.Len()),
	}
	if err := a.WriteHeader(&h); err != nil {
		return err
	}
	if _, err := io.Copy(a, t.body); err != nil {
		return err
	}
	return nil
}

func writeDebianBinaryFile(a *ar.Writer, n time.Time) error {
	h := tape.Header{
		Filename: DebBinaryFile,
		Uid:      0,
		Gid:      0,
		ModTime:  n,
		Mode:     0644,
		Length:   int64(len(DebVersion)),
	}
	if err := a.WriteHeader(&h); err != nil {
		return err
	}
	if _, err := io.WriteString(a, DebVersion); err != nil {
		return err
	}
	return nil
}

func prepareControl(c mack.Control) (*bytes.Buffer, error) {
	fs := template.FuncMap{
		"join": strings.Join,
		"indent": func(s string) string {
			var lines []string

			sc := bufio.NewScanner(strings.NewReader(s))
			sc.Split(bufio.ScanLines)
			for sc.Scan() {
				x := sc.Text()
				if x == "" {
					x = "."
				}
				lines = append(lines, " "+x)
			}
			return strings.Join(lines, "\n")
		},
	}
	t, err := template.New("control").Funcs(fs).Parse(strings.TrimSpace(control) + "\n")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, c); err != nil {
		return nil, err
	}
	return &buf, nil
}

type tarball struct {
	Name string

	body *bytes.Buffer
	zip  *gzip.Writer
	ark  *tar.Writer

	paths sort.StringSlice
}

func newTarball(n string) *tarball {
	body := new(bytes.Buffer)
	zipper := gzip.NewWriter(body)
	return &tarball{
		Name: n,
		body: body,
		zip:  zipper,
		ark:  tar.NewWriter(zipper),
	}
}

func (t *tarball) WriteString(f, c string, n time.Time) error {
	h := tar.Header{
		Name:     f,
		ModTime:  n,
		Uid:      0,
		Gid:      0,
		Mode:     0644,
		Size:     int64(len(c)),
		Typeflag: tar.TypeReg,
	}
	if err := t.ark.WriteHeader(&h); err != nil {
		return err
	}
	_, err := io.WriteString(t.ark, c)
	return err
}

func (t *tarball) WriteFile(f *mack.File, n time.Time) ([]byte, error) {
	ds, _ := filepath.Split(f.Dst)
	if err := t.WriteDirectoryTree(ds, n); err != nil {
		return nil, err
	}
	bs, err := readFile(f.Src, f.Compress)
	if err != nil {
		return nil, err
	}
	h := tar.Header{
		Name:     f.String(),
		Size:     int64(len(bs)),
		ModTime:  n,
		Mode:     f.Mode(),
		Gid:      0,
		Uid:      0,
		Typeflag: tar.TypeReg,
	}
	if err := t.ark.WriteHeader(&h); err != nil {
		return nil, err
	}
	if _, err := t.ark.Write(bs); err != nil {
		return nil, err
	}
	sum := md5.Sum(bs)
	return sum[:], nil
}

func (t *tarball) WriteDirectoryTree(ds string, n time.Time) error {
	var b string
	ix := t.paths.Search(ds)
	if n := t.paths.Len(); n > 0 && ix < n && t.paths[ix] == ds {
		return nil
	}
	for _, p := range strings.Split(ds, string(os.PathSeparator)) {
		if p == "" {
			continue
		}
		b = filepath.Join(b, p)
		ix := t.paths.Search(b)
		if n := t.paths.Len(); n > 0 && ix < n && t.paths[ix] == b {
			continue
		}
		hd := tar.Header{
			Name:     "./" + b + "/",
			ModTime:  n,
			Mode:     0755,
			Gid:      0,
			Uid:      0,
			Typeflag: tar.TypeDir,
		}
		if err := t.ark.WriteHeader(&hd); err != nil {
			return err
		}
		t.paths = append(t.paths, b)
		t.paths.Sort()
	}
	return nil
}

func (t *tarball) Close() error {
	if err := t.ark.Close(); err != nil {
		return err
	}
	return t.zip.Close()
}

func readFile(p string, z bool) ([]byte, error) {
	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if z {
		b := new(bytes.Buffer)
		g := gzip.NewWriter(b)
		if _, err := g.Write(bs); err != nil {
			return nil, err
		}
		if err := g.Close(); err != nil {
			return nil, err
		}
		bs = b.Bytes()
	}
	return bs, nil
}
