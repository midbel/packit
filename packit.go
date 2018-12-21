package packit

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/midbel/tape/ar"
)

const (
	defaultEtcDir = "etc/"
	defaultDocDir = "usr/share/doc"
	defaultBinDir = "usr/bin"
)

const (
	defaultHost  = "localhost.localdomain"
	defaultUser  = "root"
	defaultGroup = "root"
)

type Package interface {
	PackageName() string
	Build(w io.Writer) error
	Metadata() *Makefile
}

type Signature struct {
	Payload int64
	Size    int64
	Sha1    string
	Sha256  string
	MD5     string
}

type Builder interface {
	PackageName() string
	Build(w io.Writer) error
}

type Makefile struct {
	*Control `toml:"metadata"`
	Files    []*File   `toml:"resource"`
	Changes  []*Change `toml:"changelog"`

	Preinst  *Script `toml:"pre-install"`
	Postinst *Script `toml:"post-install"`
	Prerm    *Script `toml:"pre-remove"`
	Postrm   *Script `toml:"post-remove"`
}

func (m *Makefile) String() string {
	if m.Control != nil {
		return m.Control.Package
	}
	return "makefile"
}

func Open(file string) (Package, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	magic := make([]byte, 64)
	if _, err := r.Read(magic); err != nil {
		return nil, err
	}
	var readPackage func(r io.Reader) (Package, error)
	if bytes.HasPrefix(magic, ar.Magic) {
		readPackage = openDEB
	} else if bytes.HasPrefix(magic, rpmMagic) {
		readPackage = openRPM
	} else {
		return nil, fmt.Errorf("unrecognized package type")
	}
	if readPackage == nil {
		return nil, fmt.Errorf("not yet implemented")
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return readPackage(r)
}

func Prepare(m *Makefile, format string) (Package, error) {
	switch format {
	case "deb", "":
		return &DEB{m}, nil
	case "rpm":
		return &RPM{m}, nil
	default:
		return nil, fmt.Errorf("unsupported package type %q", format)
	}
}

type Change struct {
	When        time.Time `toml:"date"`
	Changes     []string  `toml:"changes"`
	*Maintainer `toml:"maintainer"`
}

type Maintainer struct {
	Name  string `toml:"name"`
	Email string `toml:"email"`
}

func (m Maintainer) String() string {
	return fmt.Sprintf("%s <%s>", m.Name, m.Email)
}

type code string

func (c *code) String() string {
	return string(*c)
}

func (c *code) Set(s string) error {
	if i, err := os.Stat(s); err == nil && i.Mode().IsRegular() {
		bs, err := ioutil.ReadFile(s)
		if err != nil {
			return err
		}
		*c = code(bs)
	} else {
		*c = code(s)
	}
	return nil
}

type Script struct {
	Text code `toml:"script"`
}

func (s *Script) String() string {
	return s.Text.String()
}

func (s *Script) Valid() bool {
	if s.Text == "" {
		return false
	}
	return strings.HasPrefix(s.Text.String(), "#!")
}

type Control struct {
	Package     string   `toml:"package"`
	Version     string   `toml:"version"`
	Release     string   `toml:"release"`
	Summary     string   `toml:"summary"`
	Desc        string   `toml:"description"`
	License     string   `toml:"license"`
	Section     string   `toml:"section"`
	Priority    string   `toml:"priority"`
	Os          string   `toml:"os"`
	Arch        uint8    `toml:"arch"`
	Vendor      string   `toml:"vendor"`
	Home        string   `toml:"homepage"`
	Depends     []string `toml:"depends"`
	Suggests    []string `toml:"suggests"`
	Compiler    string   `toml:"compiler"`
	*Maintainer `toml:"maintainer"`

	Size int64 `toml:"-"`
}

func (c Control) PackageName() string {
	return fmt.Sprintf("%s-%s", c.Package, c.Version)
}

type File struct {
	Src      string `toml:"source"`
	Dst      string `toml:"destination"`
	Name     string `toml:"filename"`
	Compress bool   `toml:"compress"`
	Perm     int    `toml:"mode"`
	Conf     bool   `toml:"conf"`

	Sum  string `toml:"-"`
	Size int64  `toml:"-"`
}

func (f File) String() string {
	d, _ := filepath.Split(f.Dst)
	return filepath.Join(d, f.Filename())
}

func (f File) Mode() int64 {
	if f.Perm == 0 {
		return 0644
	}
	return int64(f.Perm)
}

func (f File) Filename() string {
	if f.Name == "" {
		return filepath.Base(f.Src)
	}
	return f.Name
}

func IsConfFile(n string) bool {
	return strings.Index(n, defaultEtcDir) >= 0
}
