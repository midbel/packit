package deb

import (
	// "archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/packit"
	"github.com/midbel/packit/text"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
	"github.com/midbel/tape/tar"
	"github.com/midbel/textwrap"
)

const (
	debVersion     = "2.0\n"
	debDataTar     = "data.tar.gz"
	debControlTar  = "control.tar.gz"
	debBinaryFile  = "debian-binary"
	debControlFile = "control"
	debSumFile     = "md5sums"
	debConfFile    = "conffiles"
	debChangeFile  = "changelog.gz"
	debCopyFile    = "copyright"
	debPreinst     = "preinst"
	debPostinst    = "postinst"
	debPrerem      = "prerm"
	debPostrem     = "postrm"
	debDateFormat  = "Mon, 02 Jan 2006 15:04:05 -0700"

	debDocDir = "usr/share/doc"

	debArchAll = "all"
	debArch64  = "amd64"
	debArch32  = "i386"
)

func Extract(file, dir string, flat, all bool) error {
	return nil
}

func Info(file string) (packit.Metadata, error) {
	return packit.Metadata{}, nil
}

func Verify(file string) error {
	return nil
}

func List(file string) ([]packit.Resource, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return nil, nil
}

func Build(dir string, meta packit.Metadata) error {
	w, err := os.Create(filepath.Join(dir, getPackageName(meta)))
	if err != nil {
		return err
	}
	defer w.Close()
	if err := createChangelog(&meta); err != nil {
		return err
	}
	if err := createLicense(&meta); err != nil {
		return err
	}
	return build(w, meta)
}

func build(w io.Writer, meta packit.Metadata) error {
	arw, err := ar.NewWriter(w)
	if err != nil {
		return err
	}

	if err := writeDebian(arw, meta); err != nil {
		return err
	}
	if err := writeControl(arw, meta); err != nil {
		return err
	}
	if err := writeData(arw, meta); err != nil {
		return err
	}
	return arw.Close()
}

func writeDebian(arw *ar.Writer, meta packit.Metadata) error {
	h := getHeader(debBinaryFile, len(debVersion), meta.Date)
	if err := arw.WriteHeader(&h); err != nil {
		return err
	}
	_, err := io.WriteString(arw, debVersion)
	return err
}

func writeControl(arw *ar.Writer, meta packit.Metadata) error {
	var (
		tmp   bytes.Buffer
		buf   = gzip.NewWriter(&tmp)
		tw    = tar.NewWriter(buf)
		files = []func(*tar.Writer, packit.Metadata) error{
			appendControlFile,
			appendChecksums,
			appendConffiles,
			appendScripts,
		}
	)
	buf.Header.Name = debControlTar
	for _, f := range files {
		if err := f(tw, meta); err != nil {
			return err
		}
	}
	tw.Close()
	buf.Close()

	h := getHeader(debControlTar, tmp.Len(), meta.Date)
	if err := arw.WriteHeader(&h); err != nil {
		return err
	}
	_, err := io.Copy(arw, &tmp)
	return err
}

func writeData(arw *ar.Writer, meta packit.Metadata) error {
	sort.Slice(meta.Resources, func(i, j int) bool {
		return meta.Resources[i].Path() < meta.Resources[j].Path()
	})
	var (
		tmp  bytes.Buffer
		buf  = gzip.NewWriter(&tmp)
		tw   = tar.NewWriter(buf)
		dirs = make(map[string]struct{})
	)
	buf.Header.Name = debDataTar
	for _, r := range meta.Resources {
		if err := appendResource(tw, r, dirs); err != nil {
			return err
		}
	}
	tw.Close()
	buf.Close()

	h := getHeader(debDataTar, tmp.Len(), meta.Date)
	if err := arw.WriteHeader(&h); err != nil {
		return err
	}
	_, err := io.Copy(arw, &tmp)
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
	h := getTarHeaderFile(res.Path(), buf.Len(), res.ModTime)
	if err := tw.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(tw, &buf)
	return err
}

func appendScripts(tw *tar.Writer, meta packit.Metadata) error {
	write := func(script, file string) error {
		if script == "" {
			return nil
		}
		if b, err := os.ReadFile(script); err == nil {
			script = string(b)
		}
		h := getTarHeaderFile(file, len(script), meta.Date)
		h.Perm = 0755
		if err := tw.WriteHeader(&h); err != nil {
			return err
		}
		_, err := io.WriteString(tw, script)
		return err
	}
	scripts := []struct {
		Script string
		File   string
	}{
		{Script: prepareScript(meta.PreInst), File: debPreinst},
		{Script: prepareScript(meta.PostInst), File: debPostinst},
		{Script: prepareScript(meta.PreRem), File: debPrerem},
		{Script: prepareScript(meta.PostRem), File: debPostrem},
	}
	for _, s := range scripts {
		if err := write(s.Script, s.File); err != nil {
			return err
		}
	}
	return nil
}

func prepareScript(s packit.Script) string {
	if s.Code == "" {
		return ""
	}
	if s.Program == "" {
		s.Program = packit.Bash
	}
	if !strings.HasPrefix(s.Code, packit.Shebang) {
		var (
			cmd, _ = exec.LookPath(s.Program)
			str    strings.Builder
		)
		str.WriteString(packit.Shebang)
		str.WriteString(cmd)
		str.WriteString("\n\n")
		str.WriteString(s.Code)
		return str.String()
	}
	return s.Code
}

func appendConffiles(tw *tar.Writer, meta packit.Metadata) error {
	var buf bytes.Buffer
	for _, r := range meta.Resources {
		if !r.IsConfig() {
			continue
		}
	}
	if buf.Len() == 0 {
		return nil
	}
	h := getTarHeaderFile(debConfFile, buf.Len(), meta.Date)
	if err := tw.WriteHeader(&h); err != nil {
		return err
	}
	_, err := io.Copy(tw, &buf)
	return err
}

func appendChecksums(tw *tar.Writer, meta packit.Metadata) error {
	var buf bytes.Buffer
	for _, r := range meta.Resources {
		fmt.Fprintf(&buf, "%s  %s\n", r.Digest, r.Path())
	}
	h := getTarHeaderFile(debSumFile, buf.Len(), meta.Date)
	if err := tw.WriteHeader(&h); err != nil {
		return err
	}
	_, err := io.Copy(tw, &buf)
	return err
}

func appendControlFile(tw *tar.Writer, meta packit.Metadata) error {
	tpl, err := template.New("control").Funcs(fmap).Parse(controlfile)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := text.Execute(tpl, &buf, meta); err != nil {
		return err
	}

	h := getTarHeaderFile(debControlFile, buf.Len(), meta.Date)
	if err := tw.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(tw, &buf)
	return err
}

func createChangelog(meta *packit.Metadata) error {
	if meta.HasChangelog() {
		return nil
	}
	tpl, err := template.New("changelog").Funcs(fmap).Parse(changefile)
	if err != nil {
		return err
	}
	var (
		tmp    bytes.Buffer
		sum    = md5.New()
		wrt    = io.MultiWriter(&tmp, sum)
		buf, _ = gzip.NewWriterLevel(wrt, gzip.BestCompression)
		file   = filepath.Join(os.TempDir(), packit.Changelog)
	)
	if err := tpl.Execute(buf, meta); err != nil {
		return err
	}
	buf.Close()
	if err := os.WriteFile(file, tmp.Bytes(), 0644); err != nil {
		return err
	}
	res := packit.Resource{
		File:    file,
		Archive: filepath.Join(debDocDir, meta.Package, debChangeFile),
		Digest:  fmt.Sprintf("%x", sum.Sum(nil)),
		Size:    int64(tmp.Len()),
		ModTime: meta.Date,
	}
	meta.Resources = append(meta.Resources, res)
	return nil
}

func createLicense(meta *packit.Metadata) error {
	if meta.HasLicense() {
		return nil
	}
	str, err := packit.GetLicense(meta.License, *meta)
	if err != nil {
		return err
	}
	file := filepath.Join(os.TempDir(), packit.License)
	if err := os.WriteFile(file, []byte(str), 0644); err != nil {
		return err
	}
	res := packit.Resource{
		File:    file,
		Archive: filepath.Join(debDocDir, meta.Package, debCopyFile),
		Digest:  fmt.Sprintf("%x", md5.Sum([]byte(str))),
		Size:    int64(len(str)),
		ModTime: meta.Date,
	}
	meta.Resources = append(meta.Resources, res)
	return nil
}

func getTarHeaderFile(file string, size int, when time.Time) tar.Header {
	return tar.Header{
		Name: file,
		Perm: 0644,
		// Mode:     0644,
		Size:    int64(size),
		ModTime: when,
		Gid:     0,
		Uid:     0,
		Type:    tar.TypeReg,
		// Typeflag: tar.TypeReg,
	}
}

func getTarHeaderDir(file string, when time.Time) tar.Header {
	return tar.Header{
		Name: file,
		Perm: 0755,
		// Mode:     0755,
		ModTime: when,
		Gid:     0,
		Uid:     0,
		Type:    tar.TypeDir,
		// Typeflag: tar.TypeDir,
	}
}

func getHeader(file string, size int, when time.Time) tape.Header {
	return tape.Header{
		Filename: file,
		Uid:      0,
		Gid:      0,
		Mode:     0644,
		Size:     int64(size),
		ModTime:  when,
	}
}

//go:embed control.tpl
var controlfile string

//go:embed changelog.tpl
var changefile string

var fmap = template.FuncMap{
	"join":      strings.Join,
	"trimspace": strings.TrimSpace,
	"datetime":  getPackageDate,
	"arch":      getPackageArch,
	"bytesize":  getPackageSize,
	"deplist":   joinDependencies,
	"wrap1":     wrapText(" "),
	"wrap2":     wrapText("  "),
}

const namepat = "%s-%s_%s.%s"

func getPackageName(meta packit.Metadata) string {
	arch := getPackageArch(meta.Arch)
	return fmt.Sprintf(namepat, meta.Package, meta.Version, arch, packit.DEB)
}

func getPackageArch(arch int) string {
	switch arch {
	case packit.Arch64:
		return debArch64
	case packit.Arch32:
		return debArch32
	default:
		return debArchAll
	}
}

func getPackageSize(size int64) int64 {
	return size >> 10
}

func getPackageDate(when time.Time) string {
	return when.Format(debDateFormat)
}

func joinDependencies(list []packit.Dependency) string {
	var str strings.Builder
	for i, d := range list {
		if i > 0 {
			str.WriteString(", ")
		}
		str.WriteString(d.Name)
		if d.Cond == 0 || d.Version == "" {
			continue
		}
		str.WriteString("(")
		var op string
		switch d.Cond {
		case packit.Eq:
			op = "="
		case packit.Lt:
			op = "<"
		case packit.Le:
			op = "<="
		case packit.Gt:
			op = ">"
		case packit.Ge:
			op = ">="
		}
		str.WriteString(op)
		str.WriteString(" ")
		str.WriteString(d.Version)
		str.WriteString(")")
	}
	return str.String()
}

func wrapText(indent string) func(string) string {
	options := []textwrap.WrapOption{
		textwrap.WithLength(80),
		textwrap.WithIndent(indent),
	}
	w := textwrap.New(options...)
	return func(str string) string {
		return w.Wrap(str)
	}
}
