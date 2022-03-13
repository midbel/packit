package apk

import (
	"bufio"
	"fmt"
	"io"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/packit"
)

const (
	ctrlPackage    = "pkgname"
	ctrlVersion    = "pkgver"
	ctrlDesc       = "pkgdesc"
	ctrlUrl        = "url"
	ctrlBuildDate  = "builddate"
	ctrlMaintainer = "packager"
	ctrlSize       = "size"
	ctrlArch       = "arch"
	ctrlOrigin     = "origin"
	ctrlLicense    = "license"
	ctrlDepend     = "depend"
	ctrlReplace    = "replace"
	ctrlProvide    = "provide"
	ctrlHash       = "datahash"
)

func ParseControl(r io.Reader) (packit.Metadata, error) {
	var meta packit.Metadata
	return meta, parseControl(r, &meta)
}

func parseControl(r io.Reader, meta *packit.Metadata) error {
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		key, value, ok := strings.Cut(scan.Text(), "=")
		if !ok {
			return fmt.Errorf("invalid line: %q", scan.Text())
		}
		switch value := strings.TrimSpace(value); strings.TrimSpace(key) {
		case ctrlPackage:
			meta.Package = value
		case ctrlVersion:
			meta.Version = value
		case ctrlDesc:
			meta.Summary = value
		case ctrlUrl:
			meta.Home = value
		case ctrlBuildDate:
			unix, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			meta.Date = time.Unix(unix, 0)
		case ctrlMaintainer:
			addr, err := mail.ParseAddress(value)
			meta.Maintainer.Name = value
			if err == nil {
				meta.Maintainer.Name = addr.Name
				meta.Maintainer.Email = addr.Address
			}
		case ctrlSize:
			size, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			meta.Size = size / 1000
		case ctrlArch:
		case ctrlOrigin:
		case ctrlLicense:
			meta.License = value
		case ctrlDepend:
		case ctrlReplace:
		case ctrlProvide:
		case ctrlHash:
			meta.DataHash = value
		default:
		}
	}
	return scan.Err()
}
