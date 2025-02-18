package deb

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/midbel/packit"
	"github.com/midbel/packit/deb/control"
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
)

func Build(mf *packit.Makefile) (packit.Builder, error) {
	if mf == nil {
		return nil, fmt.Errorf("empty makefile")
	}
	b := builder{
		when:    time.Now(),
		control: mf.Control,
		files:   mf.Files,
		changes: mf.Changes,
	}
	return &b, nil
}

func Open(file string) (packit.Package, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, p, err := openFile(f)
	if err != nil {
		return nil, err
	}
	if err := readData(r, p); err != nil {
		return nil, err
	}
	return p, nil
}

func openFile(f *os.File) (tape.Reader, *pkg, error) {
	r, err := ar.NewReader(f)
	if err != nil {
		return nil, nil, err
	}
	if err := readDebian(r); err != nil {
		return nil, nil, err
	}
	p := pkg{name: filepath.Base(f.Name())}
	if err := readControl(r, &p); err != nil {
		return nil, nil, err
	}
	return r, &p, nil
}

func About(file string) (*packit.Control, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	_, p, err := openFile(f)
	if err != nil {
		return nil, err
	}
	return control.Parse(p.control)
}

func Arch(a uint8) string {
	switch a {
	case packit.Arch32:
		return "i386"
	case packit.Arch64:
		return "amd64"
	case packit.ArchAll:
		return "all"
	default:
		return "unknown"
	}
}
