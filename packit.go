package packit

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

var (
	ErrUnsupportedPayloadFormat = errors.New("unsupported payload format")
	ErrMalformedPackage         = errors.New("malformed package")
)

var ErrSkip = errors.New("skip")

const (
	ExtGZ = ".gz"
	ExtXZ = ".xz"
)

const (
	defaultEtcDir = "etc/"
	defaultDocDir = "usr/share/doc"
	defaultBinDir = "usr/bin"
)

const (
	DefaultDistrib = "unstable"
	DefaultOS      = "linux"
	DefaultHost    = "localhost.localdomain"
	DefaultUser    = "root"
	DefaultGroup   = "root"
)

const (
	Arch32  = 32
	Arch64  = 64
	ArchAll = 0
)

var DefaultMaintainer Maintainer

func init() {
	DefaultMaintainer = Maintainer{
		Name:  os.Getenv("PACKIT_MAINTAINER_NAME"),
		Email: os.Getenv("PACKIT_MAINTAINER_EMAIL"),
	}
}

type Package interface {
	PackageName() string
	PackageType() string
	About() Control
	History() History
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

type Maintainer struct {
	Name  string `toml:"name"`
	Email string `toml:"email"`
}

func (m *Maintainer) String() string {
	if m == nil || m.Name == "" || m.Email == "" {
		return "nobody <unknown>"
	}
	return fmt.Sprintf("%s <%s>", m.Name, m.Email)
}

func ParseMaintainer(s string) (*Maintainer, error) {
	m, _, err := parseMaintainer(s, false)
	return m, err
}

func ParseMaintainerVersion(s string) (*Maintainer, string, error) {
	return parseMaintainer(s, true)
}

func parseMaintainer(s string, version bool) (*Maintainer, string, error) {
	consume := func(r io.RuneScanner, char rune, chk func(rune) bool) (string, error) {
		// if chk == nil {
		// 	chk = func(_ rune) bool { return true }
		// }
		var str bytes.Buffer
		for {
			r, _, err := r.ReadRune()
			if err == io.EOF || r == 0 || r == char {
				break
			}
			if err != nil {
				return "", err
			}
			if !chk(r) {
				return "", fmt.Errorf("illegal token %c", r)
			}
			str.WriteRune(r)
		}
		return strings.TrimSpace(str.String()), nil
	}
	checkNameRunes := func(r rune) bool {
		return unicode.IsLetter(r) || r == ' ' || r == '-'
	}
	checkEmailRunes := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' || r == '@'
	}
	checkVersionRunes := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-'
	}

	var (
		m   Maintainer
		v   string
		err error
	)
	r := strings.NewReader(s)
	if m.Name, err = consume(r, '<', checkNameRunes); err != nil {
		return nil, "", fmt.Errorf("fail parsing maintainer name: %v", err)
	}
	if m.Email, err = consume(r, '>', checkEmailRunes); err != nil {
		return nil, "", fmt.Errorf("fail parsing maintainer e-mail: %v", err)
	}
	if version {
		for k, _, err := r.ReadRune(); k == ' ' || k == '-'; k, _, err = r.ReadRune() {
			if err != nil || k == 0 {
				return nil, "", fmt.Errorf("fail parsing version")
			}
		}
		r.UnreadRune()
		if v, err = consume(r, 0, checkVersionRunes); err != nil {
			return nil, "", fmt.Errorf("fail parsing version: %v", err)
		}
	}
	if m.Name == "" {
		return nil, "", fmt.Errorf("missing maintainer name")
	}
	if m.Email == "" {
		return nil, "", fmt.Errorf("missing maintainer e-mail")
	}
	return &m, v, err
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
	Package     string `toml:"package"`
	Version     string `toml:"version"`
	Release     string `toml:"release"`
	Summary     string `toml:"summary"`
	Desc        string `toml:"description"`
	License     string `toml:"license"`
	Section     string `toml:"section"`
	Priority    string `toml:"priority"`
	Os          string `toml:"os"`
	Arch        uint8  `toml:"arch"`
	Vendor      string `toml:"vendor"`
	Home        string `toml:"homepage"`
	*Maintainer `toml:"maintainer"`

	Depends   []string `toml:"depends"`
	Suggests  []string `toml:"suggests"`
	Provides  []string `toml:"provides"`
	Breaks    []string `toml:"breaks"`
	Conflicts []string `toml:"conflicts"`
	Replaces  []string `toml:"replaces"`

	Compiler string `toml:"compiler"`

	Format string    `toml:"-"`
	Status string    `toml:"-"`
	Source string    `toml:"-"`
	Date   time.Time `toml:"-"`
	Size   int64     `toml:"-"`
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

	Conf    bool   `toml:"conf"`
	Doc     bool   `toml:"doc"`
	License bool   `toml:"license"`
	Readme  bool   `toml:"readme"`
	Lang    string `toml:"lang"`

	Sum  string `toml:"-"`
	Size int64  `toml:"-"`
}

func LocalFile(p string) (*File, error) {
	i, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if i.Name() == "." || i.IsDir() {
		return nil, ErrSkip
	}
	f := File{
		Src:  p,
		Dst:  p,
		Name: filepath.Base(p),
		Perm: int(i.Mode()),
		Conf: IsConfFile(p),
	}
	return &f, nil
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
