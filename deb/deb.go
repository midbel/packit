package deb

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/packit"
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
	debChangeFile  = "changelog.gz"
	debPreinst     = "preinst"
	debPostinst    = "postinst"
	debPrerem      = "prerm"
	debPostrem     = "postrm"
	debDateFormat  = "Mon, 02 Jan 2006 15:04:05 -0700"

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
	defer arw.Close()

	if err := writeDebian(arw, meta); err != nil {
		return err
	}
	if err := writeControl(arw, meta); err != nil {
		return err
	}
	return nil
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
	r, err := getControlFile(meta)
	if err != nil {
		return err
	}
	io.Copy(os.Stdout, r)
  for _, r := range meta.Resources {
    io.WriteString(os.Stdout, fmt.Sprintf("%s %s\n", r.Digest, r.File))
  }
  for _, r := range meta.Resources {
    if !r.IsConfig() {
      continue
    }
  }
	return nil
}

func writeData(arw *ar.Writer, meta packit.Metadata) error {
	return nil
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

var fmap = template.FuncMap{
	"join":     strings.Join,
	"datetime": getPackageDate,
	"arch":     getPackageArch,
	"bytesize": getPackageSize,
}

func getControlFile(meta packit.Metadata) (io.Reader, error) {
	tpl, err := template.New("control").Funcs(fmap).Parse(controlfile)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, meta); err != nil {
		return nil, err
	}
	return &buf, nil
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
