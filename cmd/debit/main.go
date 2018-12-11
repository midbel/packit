package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/mack"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
	"github.com/midbel/toml"
	"golang.org/x/sync/errgroup"
)

const (
	Version     = "2.0\n"
	DataTar     = "data.tar.gz"
	ControlTar  = "control.tar.gz"
	BinaryFile  = "debian-binary"
	ControlFile = "./control"
	SumFile     = "./md5sums"
	ConfFile    = "./conffiles"
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
Depends: {{join .Depends ", "}}
Suggests: {{join .Suggests ", "}}
Installed-Size: {{.Size}}
Build-Using: {{.Compiler}}
Description: {{.Summary}}
{{indent .Desc}}
`

const changelog = `{{range .Changes}}  {{$.Package}} ({{$.Version}}) unstable; urgency=low

{{range .Changes}}   * {{.}}
{{end}}
  -- {{.Maintainer.Name}} <{{.Maintainer.Email}}> {{strftime .When}}
{{end}}`

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)
}

type makefile struct {
	Type    string         `toml:"type"`
	Control *mack.Control  `toml:"metadata"`
	Files   []*mack.File   `toml:"resource"`
	Changes []*mack.Change `toml:"changelog"`

	Preinst  *mack.Script `toml:"pre-install"`
	Postinst *mack.Script `toml:"post-install"`
	Prerm    *mack.Script `toml:"pre-remove"`
	Postrm   *mack.Script `toml:"post-remove"`
}

func main() {
	datadir := flag.String("d", os.TempDir(), "datadir")
	flag.Parse()

	if err := os.MkdirAll(*datadir, 0755); err != nil && !os.IsExist(err) {
		log.Fatalln(err)
	}

	var group errgroup.Group
	for _, a := range flag.Args() {
		a := a
		group.Go(func() error {
			r, err := os.Open(a)
			if err != nil {
				return err
			}
			defer r.Close()

			var mf makefile
			if err := toml.NewDecoder(r).Decode(&mf); err != nil {
				return err
			}
			if mf.Type != "" && mf.Type != "binary" {
				return fmt.Errorf("can not create deb for type %s", mf.Type)
			}
			w, err := os.Create(filepath.Join(*datadir, mf.Control.PackageName()+".deb"))
			if err != nil {
				return err
			}
			defer w.Close()
			return buildPackage(w, &mf)
		})
	}
	if err := group.Wait(); err != nil {
		log.Fatalln(err)
	}
}

func buildPackage(w io.Writer, mf *makefile) error {
	aw, err := ar.NewWriter(w)
	if err != nil {
		return err
	}
	defer aw.Close()

	if err := writeDebian(aw); err != nil {
		return err
	}
	var data, control bytes.Buffer
	if err := writeData(&data, mf); err != nil {
		return err
	}
	if err := writeControl(&control, mf); err != nil {
		return err
	}
	ts := []struct {
		File   string
		Buffer bytes.Buffer
	}{
		{File: ControlTar, Buffer: control},
		{File: DataTar, Buffer: data},
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

func writeControl(w io.Writer, mf *makefile) error {
	var sums, confs []string
	for _, f := range mf.Files {
		if f.Conf {
			n := f.String()
			if !strings.HasPrefix(n, "/") {
				n = "/" + n
			}
			confs = append(confs, n)
		}
		sums = append(sums, fmt.Sprintf("%s %s", f.Sum, f.String()))
		mf.Control.Size += f.Size
	}
	wt := tar.NewWriter(w)

	if err := writeControlFile(wt, mf.Control); err != nil {
		return err
	}
	fs := []struct {
		File string
		Data []string
	}{
		{File: SumFile, Data: sums},
		{File: ConfFile, Data: confs},
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
	if err := writeScriptFiles(wt, mf); err != nil {
		return err
	}
	return wt.Close()
}

func writeScriptFiles(w *tar.Writer, mf *makefile) error {
	ms := []struct {
		File string
		*mack.Script
	}{
		{File: "preinst", Script: mf.Preinst},
		{File: "postinst", Script: mf.Postinst},
		{File: "prerm", Script: mf.Prerm},
		{File: "postrm", Script: mf.Postrm},
	}
	for _, s := range ms {
		if s.Script == nil || !s.Valid() {
			continue
		}
		var body bytes.Buffer
		io.WriteString(&body, s.String())

		h := tar.Header{
			Name:     s.File,
			Size:     int64(body.Len()),
			Uid:      0,
			Gid:      0,
			ModTime:  time.Now().Truncate(time.Minute),
			Typeflag: tar.TypeReg,
			Mode:     0755,
		}
		if err := w.WriteHeader(&h); err != nil {
			return err
		}
		if _, err := io.Copy(w, &body); err != nil {
			return err
		}
	}
	return nil
}

func writeControlFile(w *tar.Writer, c *mack.Control) error {
	fs := template.FuncMap{
		"join": strings.Join,
		"indent": func(v string) string {
			var lines []string

			s := bufio.NewScanner(strings.NewReader(v))
			for s.Scan() {
				x := s.Text()
				if x == "" {
					x = "."
				}
				lines = append(lines, " "+x)
			}
			return strings.Join(lines, "\n")
		},
	}
	var body bytes.Buffer
	t, err := template.New("control").Funcs(fs).Parse(strings.TrimSpace(control) + "\n")
	if err != nil {
		return err
	}
	c.Size = c.Size >> 10
	if err := t.Execute(&body, *c); err != nil {
		return err
	}
	h := tar.Header{
		Name:     ControlFile,
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

func writeData(w io.Writer, mf *makefile) error {
	wt := tar.NewWriter(w)

	digest := md5.New()
	done := make(map[string]struct{})
	if len(mf.Changes) > 0 {
		var body bytes.Buffer
		for _, g := range mf.Changes {
			if g.Maintainer == nil {
				g.Maintainer = mf.Control.Maintainer
			}
		}
		z, _ := gzip.NewWriterLevel(&body, gzip.BestCompression)
		fs := template.FuncMap{
			"strftime": func(t time.Time) string { return t.Format("Mon, 02 Jan 2006 15:04:05 -0700") },
		}
		t, err := template.New("changelog").Funcs(fs).Parse(changelog)
		if err != nil {
			return err
		}
		i := struct {
			Package string
			Version string
			Changes []*mack.Change
		}{
			Package: mf.Control.Package,
			Version: mf.Control.Version,
			Changes: mf.Changes,
		}
		if err := t.Execute(z, i); err != nil {
			return err
		}
		if err := z.Close(); err != nil {
			return err
		}
		name := filepath.Join("usr/share/doc", mf.Control.Package, "changelog.gz")
		if done, err = intermediateDirectories(wt, name, done); err != nil {
			return err
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
		if err := wt.WriteHeader(&h); err != nil {
			return err
		}
		if n, err := io.Copy(io.MultiWriter(wt, digest), &body); err != nil {
			return err
		} else {
			f := mack.File{
				Name: h.Name,
				Size: int64(n),
				Sum:  fmt.Sprintf("%x", digest.Sum(nil)),
			}
			mf.Files = append(mf.Files, &f)
		}
		digest.Reset()
	}
	for _, i := range mf.Files {
		if i.Src == "" && i.Dst == "" {
			continue
		}
		f, err := os.Open(i.Src)
		if err != nil {
			return err
		}
		s, err := f.Stat()
		if err != nil {
			return err
		}
		if done, err = intermediateDirectories(wt, i.String(), done); err != nil {
			return err
		}
		r := io.TeeReader(f, digest)
		h := tar.Header{
			Name:     i.String(),
			Mode:     i.Mode(),
			Size:     s.Size(),
			ModTime:  s.ModTime(),
			Gid:      0,
			Uid:      0,
			Typeflag: tar.TypeReg,
		}
		if err := wt.WriteHeader(&h); err != nil {
			return err
		}
		if i.Size, err = io.Copy(wt, r); err != nil {
			return err
		}
		i.Sum = fmt.Sprintf("%x", digest.Sum(nil))

		f.Close()
		digest.Reset()
	}
	return wt.Close()
}

func writeDebian(w *ar.Writer) error {
	h := tape.Header{
		Filename: BinaryFile,
		Uid:      0,
		Gid:      0,
		ModTime:  time.Now().Truncate(time.Minute),
		Mode:     0644,
		Length:   int64(len(Version)),
	}
	if err := w.WriteHeader(&h); err != nil {
		return err
	}
	if _, err := io.WriteString(w, Version); err != nil {
		return err
	}
	return nil
}

func intermediateDirectories(w *tar.Writer, n string, done map[string]struct{}) (map[string]struct{}, error) {
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
