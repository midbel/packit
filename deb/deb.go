package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/packit"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
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

func Build(dir string, meta packit.Metadata) error {
	w, err := os.Create(filepath.Join(dir, getPackageName(meta)))
	if err != nil {
		return err
	}
	defer w.Close()
	if len(meta.Changes) > 0 {
		var (
			file = filepath.Join(dir, "changelog")
			err  = createChangelog(file, meta)
		)
		if err != nil {
			return err
		}
		res := packit.Resource{
			File:     file,
			Dir:      filepath.Join(debDocDir, meta.Package),
			Perm:     0644,
			Compress: true,
		}
		meta.Resources = append(meta.Resources, res)
		defer os.Remove(file)
	}
	if err := meta.Update(); err != nil {
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
		{Script: meta.PreInst, File: debPreinst},
		{Script: meta.PostInst, File: debPostinst},
		{Script: meta.PreRem, File: debPrerem},
		{Script: meta.PostRem, File: debPostrem},
	}
	for _, s := range scripts {
		if err := write(s.Script, s.File); err != nil {
			return err
		}
	}
	return nil
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

func createChangelog(file string, meta packit.Metadata) error {
	tpl, err := template.New("changelog").Funcs(fmap).Parse(changefile)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, meta); err != nil {
		return err
	}
	return os.WriteFile(file, buf.Bytes(), 0644)
}

func appendControlFile(tw *tar.Writer, meta packit.Metadata) error {
	tpl, err := template.New("control").Funcs(fmap).Parse(controlfile)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, meta); err != nil {
		return err
	}

	h := getTarHeaderFile(debControlFile, buf.Len(), meta.Date)
	if err := tw.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(tw, &buf)
	return err
}

func getTarHeaderFile(file string, size int, when time.Time) tar.Header {
	return tar.Header{
		Name:     file,
		Mode:     0644,
		Size:     int64(size),
		ModTime:  when,
		Gid:      0,
		Uid:      0,
		Typeflag: tar.TypeReg,
	}
}

func getTarHeaderDir(file string, when time.Time) tar.Header {
	return tar.Header{
		Name:     file,
		Mode:     0755,
		ModTime:  when,
		Gid:      0,
		Uid:      0,
		Typeflag: tar.TypeDir,
	}
}

func getHeader(file string, size int, when time.Time) tape.Header {
	return tape.Header{
		Filename: file,
		Uid:      0,
		Gid:      0,
		Mode:     0644,
		Length:   int64(size),
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
	"wrap1":     wrapText(" "),
	"wrap2":     wrapText("  "),
}

func getMD5File(meta packit.Metadata) (io.Reader, error) {
	return nil, nil
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
