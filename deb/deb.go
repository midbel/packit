package deb

import (
	"fmt"
	"time"

	"github.com/midbel/packit"
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
	return nil, nil
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
