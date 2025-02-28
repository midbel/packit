package deb

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/midbel/packit/internal/build"
	"github.com/midbel/packit/internal/packfile"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
	"github.com/midbel/textwrap"
)

const (
	DataFile      = "data.tar.gz"
	ControlFile   = "control.tar.gz"
	controlFile   = "control"
	debianFile    = "debian-binary"
	md5File       = "md5sums"
	confFile      = "conffiles"
	preinstFile   = "preinst"
	postinstFile  = "postinst"
	prermFile     = "prerm"
	postrmFile    = "postrm"
	changelogFile = "changelog.gz"
	copyrightFile = "copyright"
)

const debVersion = "2.0\n"

//go:embed templates/control.tpl
var aboutFile string

//go:embed templates/changelog.tpl
var changeFile string

type debBuilder struct {
	writer *ar.Writer
}

func Build(w io.Writer) (build.Builder, error) {
	wr, err := ar.NewWriter(w)
	if err != nil {
		return nil, err
	}
	b := debBuilder{
		writer: wr,
	}
	return &b, nil
}

func (d debBuilder) Build(p *packfile.Package) error {
	if err := d.setup(p); err != nil {
		return err
	}
	if err := d.build(p); err != nil {
		return err
	}
	return d.teardown(p)
}

func (d debBuilder) setup(pkg *packfile.Package) error {
	if pkg.Setup == "" {
		return nil
	}
	scan := bufio.NewScanner(strings.NewReader(pkg.Setup))
	for scan.Scan() {
		cmd := exec.Command("sh", "-c", scan.Text())
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (d debBuilder) teardown(pkg *packfile.Package) error {
	if pkg.Teardown == "" {
		return nil
	}
	scan := bufio.NewScanner(strings.NewReader(pkg.Teardown))
	for scan.Scan() {
		cmd := exec.Command("sh", "-c", scan.Text())
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (d debBuilder) build(p *packfile.Package) error {
	defer func() {
		os.Remove(ControlFile)
		os.Remove(DataFile)
	}()
	data, err := writeFiles(p)
	if err != nil {
		return err
	}
	defer data.Close()

	ctrl, err := writeControl(p)
	if err != nil {
		return err
	}
	defer ctrl.Close()

	if err := d.writeDebian(); err != nil {
		return err
	}
	if err := d.writeControl(ctrl); err != nil {
		return err
	}
	return d.writeData(data)
}

func (d debBuilder) writeDebian() error {
	h := tape.Header{
		Filename: debianFile,
		Uid:      0,
		Gid:      0,
		Mode:     0644,
		Size:     int64(len(debVersion)),
		ModTime:  time.Now(),
	}
	if err := d.writer.WriteHeader(&h); err != nil {
		return err
	}
	_, err := io.WriteString(d.writer, debVersion)
	return err
}

func (d debBuilder) writeControl(r *os.File) error {
	if _, err := r.Seek(0, os.SEEK_SET); err != nil {
		return err
	}
	h, err := tape.FileInfoHeaderFromFile(r)
	if err != nil {
		return err
	}
	if err := d.writer.WriteHeader(h); err != nil {
		return err
	}
	_, err = io.Copy(d.writer, r)
	return err
}

func (d debBuilder) writeData(r *os.File) error {
	if _, err := r.Seek(0, os.SEEK_SET); err != nil {
		return err
	}
	h, err := tape.FileInfoHeaderFromFile(r)
	if err != nil {
		return err
	}
	if err := d.writer.WriteHeader(h); err != nil {
		return err
	}
	_, err = io.Copy(d.writer, r)
	return err
}

func (d debBuilder) Close() error {
	return d.writer.Close()
}

func writeSpec(w *tar.Writer, pkg *packfile.Package) error {
	var buf bytes.Buffer

	fn := template.FuncMap{
		"fmtdesc":    formatPackageDesc,
		"fmtsize":    formatPackageSize,
		"dependency": formatDependency,
	}

	t, err := template.New("control").Funcs(fn).Parse(aboutFile)
	if err != nil {
		return err
	}
	if err := t.Execute(&buf, pkg); err != nil {
		return err
	}

	var prev rune
	str := strings.Map(func(r rune) rune {
		if prev == '\n' && r == prev {
			return -1
		}
		prev = r
		return r
	}, buf.String())

	h := makeTarHeader(controlFile, len(str), packfile.PermFile)
	if err := w.WriteHeader(h); err != nil {
		return err
	}
	_, err = io.WriteString(w, str)
	return err
}

func writeChecksums(w *tar.Writer, pkg *packfile.Package) error {
	if len(pkg.Files) == 0 {
		return nil
	}
	var str bytes.Buffer
	for _, r := range pkg.Files {
		io.WriteString(&str, fmt.Sprintf("%s  %s\n", r.Hash, r.Target))
	}

	h := makeTarHeader(md5File, str.Len(), packfile.PermFile)
	if err := w.WriteHeader(h); err != nil {
		return err
	}
	_, err := io.Copy(w, &str)
	return err
}

func writeConffiles(w *tar.Writer, pkg *packfile.Package) error {
	if len(pkg.Files) == 0 {
		return nil
	}
	var str bytes.Buffer
	for _, r := range pkg.Files {
		if !r.Config {
			continue
		}
		target := strings.TrimPrefix(r.Target, "/")
		io.WriteString(&str, fmt.Sprintf("/%s\n", target))
	}
	if str.Len() == 0 {
		return nil
	}
	h := makeTarHeader(confFile, str.Len(), packfile.PermFile)
	if err := w.WriteHeader(h); err != nil {
		return err
	}
	_, err := io.Copy(w, &str)
	return err
}

func writeControl(pkg *packfile.Package) (*os.File, error) {
	f, err := os.Create(ControlFile)
	if err != nil {
		return nil, err
	}

	ws := gzip.NewWriter(f)
	defer ws.Close()

	w := tar.NewWriter(ws)
	defer w.Close()

	if err := writeSpec(w, pkg); err != nil {
		f.Close()
		return nil, err
	}
	if err := writeChecksums(w, pkg); err != nil {
		f.Close()
		return nil, err
	}
	if err := writeConffiles(w, pkg); err != nil {
		f.Close()
		return nil, err
	}
	if err := writeScripts(w, pkg); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func writeScripts(w *tar.Writer, pkg *packfile.Package) error {
	write := func(script, file string) error {
		if script == "" {
			return nil
		}
		h := makeTarHeader(file, len(script), packfile.PermExec)
		if err := w.WriteHeader(h); err != nil {
			return err
		}
		_, err := io.WriteString(w, script)
		return err
	}

	list := []struct {
		Script string
		File   string
	}{
		{
			Script: pkg.PreInst,
			File:   preinstFile,
		},
		{
			Script: pkg.PostInst,
			File:   postinstFile,
		},
		{
			Script: pkg.PreRem,
			File:   prermFile,
		},
		{
			Script: pkg.PostRem,
			File:   postrmFile,
		},
	}
	for _, i := range list {
		if err := write(i.Script, i.File); err != nil {
			return err
		}
	}
	return nil
}

func writeFiles(pkg *packfile.Package) (*os.File, error) {
	f, err := os.Create(DataFile)
	if err != nil {
		return nil, err
	}

	ws, _ := gzip.NewWriterLevel(f, gzip.BestCompression)
	defer func() {
		ws.Flush()
		ws.Close()
	}()

	w := tar.NewWriter(ws)
	defer w.Close()

	slices.SortFunc(pkg.Files, func(a, b packfile.Resource) int {
		return strings.Compare(a.Target, b.Target)
	})
	if len(pkg.Changes) > 0 {
		res, err := writeChangelog(pkg)
		if err != nil {
			f.Close()
			return nil, err
		}
		pkg.Files = append(pkg.Files, res)
	}
	seen := make(map[string]struct{})
	for i, r := range pkg.Files {
		if r.Target == "" {
			continue
		}
		dir := filepath.Dir(r.Target)
		if _, ok := seen[dir]; len(dir) > 0 && !ok {
			paths := strings.Split(dir, string(filepath.Separator))
			for i := range paths {
				if paths[i] == "" {
					continue
				}
				target := filepath.Join(paths[:i+1]...)
				if _, ok := seen[target]; ok {
					continue
				}
				seen[target] = struct{}{}
				h := makeTarHeaderDir(strings.Join(paths[:i+1], "/"))
				if err := w.WriteHeader(h); err != nil {
					f.Close()
					return nil, err
				}
			}
		}
		if r.Compress {
			// TODO
		}
		var (
			sum  = md5.New()
			perm = packfile.GetPermissionFromPath(r.Target)
			h    = makeTarHeader(r.Target, int(r.Size), int(perm))
		)
		if err := w.WriteHeader(h); err != nil {
			f.Close()
			return nil, err
		}
		if _, err := io.Copy(io.MultiWriter(w, sum), r.Local); err != nil {
			f.Close()
			return nil, err
		}
		r.Local.Close()
		r.Hash = fmt.Sprintf("%x", sum.Sum(nil))
		pkg.Files[i] = r
	}
	return f, nil
}

func writeChangelog(pkg *packfile.Package) (packfile.Resource, error) {
	res := packfile.Resource{
		Target:  pkg.GetDirDoc(changelogFile),
		Perm:    packfile.PermFile,
		Lastmod: time.Now(),
	}

	tpl, err := template.New("changelog").Parse(changeFile)
	if err != nil {
		return res, err
	}

	f, err := os.CreateTemp("", changelogFile)
	if err != nil {
		return res, err
	}
	w, _ := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err := tpl.Execute(w, pkg); err != nil {
		return res, err
	}
	w.Flush()
	if err := w.Close(); err != nil {
		return res, err
	}
	s, _ := f.Stat()
	res.Size = s.Size()
	res.Local = f
	f.Seek(0, os.SEEK_SET)
	return res, nil
}

func makeTarHeaderDir(file string) *tar.Header {
	h := makeTarHeader(file, 0, packfile.PermDir)
	h.Typeflag = tar.TypeDir
	return h
}

func makeTarHeader(file string, size, perm int) *tar.Header {
	h := tar.Header{
		Typeflag: tar.TypeReg,
		Name:     file,
		Size:     int64(size),
		Uid:      0,
		Gid:      0,
		ModTime:  time.Now(),
		Mode:     int64(perm),
	}
	h.ChangeTime = h.ModTime
	h.AccessTime = h.ModTime
	return &h
}

func formatPackageSize(size int64) int64 {
	return size / 1000
}

func formatPackageDesc(str string) string {
	var (
		wr bytes.Buffer
		rd = bytes.NewBufferString(textwrap.Wrap(str))
		sc = bufio.NewScanner(rd)
	)
	for sc.Scan() {
		line := sc.Text()
		wr.WriteRune(' ')
		if line == "" {
			wr.WriteRune('.')
		} else {
			wr.WriteString(line)
		}
		wr.WriteRune('\n')
	}
	return wr.String()
}

func formatDependency(dp packfile.Dependency) string {
	var str strings.Builder
	str.WriteString(dp.Package)
	if dp.Arch != "" {
		str.WriteRune(':')
		str.WriteString(dp.Arch)
	}
	if dp.Version != "" {
		str.WriteRune(' ')
		str.WriteRune('(')
		str.WriteString(formatDependencyConstraint(dp.Constraint))
		str.WriteRune(' ')
		str.WriteString(dp.Version)
		str.WriteRune(')')
	}
	return str.String()
}

func formatDependencyConstraint(op string) string {
	switch op {
	case packfile.ConstraintEq:
		op = "="
	case packfile.ConstraintNe:
		op = "!="
	case packfile.ConstraintGt:
		op = ">"
	case packfile.ConstraintGe, "":
		op = ">="
	case packfile.ConstraintLt:
		op = "<"
	case packfile.ConstraintLe:
		op = "<="
	default:
		op = ""
	}
	return op
}
