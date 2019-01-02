package packit

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var UnsupportedPayloadFormat = errors.New("unsupported payload format")

const (
	defaultEtcDir = "etc/"
	defaultDocDir = "usr/share/doc"
	defaultBinDir = "usr/bin"
)

const (
	DefaultHost  = "localhost.localdomain"
	DefaultUser  = "root"
	DefaultGroup = "root"
)

const (
	Arch32  = 32
	Arch64  = 64
	ArchAll = 0
)

type Package interface {
	PackageName() string
	PackageType() string
	About() Control
	Filenames() ([]string, error)
	Resources() ([]Resource, error)
	Valid() error
	Extract(string, bool) error
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

func ArchString(a uint8) string {
	switch a {
	case Arch32:
		return "i386"
	case Arch64:
		return "x86_64"
	default:
		return "noarch"
	}
}

func Hostname() string {
	h, err := os.Hostname()
	if err == nil {
		return h
	}
	return DefaultHost
}

type Change struct {
	When        time.Time `toml:"date"`
	Body        string    `toml:"description"`
	Version     string    `toml:'version'`
	Distrib     []string  `toml:"distrib"`
	Changes     []string  `toml:"changes"`
	*Maintainer `toml:"maintainer"`
}

type Maintainer struct {
	Name  string `toml:"name"`
	Email string `toml:"email"`
}

func (m *Maintainer) String() string {
	if m == nil {
		return "<unknown>"
	}
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

	Date time.Time `toml:"-"`
	Size int64     `toml:"-"`
}

func (c Control) PackageName() string {
	return fmt.Sprintf("%s-%s", c.Package, c.Version)
}

type Resource struct {
	Name    string
	Size    int64
	Perm    int64
	ModTime time.Time
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
