package packit

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/fig"
)

const (
	EnvArchive = "archive"
)

const (
	DEB = "deb"
	RPM = "rpm"
)

const (
	docDir = "doc"
	etcDir = "etc"
)

const (
	Changelog = "CHANGELOG"
	License   = "LICENSE"
	Readme    = "README"
)

const (
	Arch64 = 64
	Arch32 = 32
)

const (
	DefaultVersion  = "0.1.0"
	DefaultLicense  = "mit"
	DefaultSection  = "contrib"
	DefaultPriority = "extra"
	DefaultOS       = "linux"
	DefaultShebang  = "#!/bin/sh"
)

const (
	EnvMaintainerName = "PACKIT_MAINTAINER_NAME"
	EnvMaintainerMail = "PACKIT_MAINTAINER_MAIL"
)

//go:embed licenses/*tpl
var licenses embed.FS

func GetLicense(name string, meta Metadata) (string, error) {
	name = fmt.Sprintf("%s.tpl", name)
	b, err := fs.ReadFile(licenses, filepath.Join("licenses", name))
	if err != nil {
		return "", err
	}
	t, err := template.New("license").Parse(string(b))
	if err != nil {
		return "", err
	}
	ctx := struct {
		Year   int
		Holder string
	}{
		Year:   meta.Date.Year(),
		Holder: meta.Maintainer.Name,
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type Metadata struct {
	Package  string
	Version  string
	Release  string
	Summary  string
	Desc     string `fig:"description"`
	License  string
	Section  string
	Priority string
	OS       string
	Arch     int
	Vendor   string
	Home     string `fig:"homepage"`
	Compiler string

	Maintainer Maintainer

	Resources []Resource `fig:"resource"`
	Changes   []Change   `fig:"change"`

	Essential bool
	Depends   []string `fig:"depend"`
	Suggests  []string `fig:"suggest"`
	Provides  []string `fig:"provide"`
	Breaks    []string `fig:"break"`
	Conflicts []string `fig:"conflict"`
	Replaces  []string `fig:"replace"`

	PreInst  Script `fig:"pre-install"`
	PostInst Script `fig:"post-install"`
	PreRem   Script `fig:"pre-remove"`
	PostRem  Script `fig:"post-remove"`

	Date time.Time `fig:"-"`
	Size int64     `fig:"-"`
}

func Load(r io.Reader, kind string) (Metadata, error) {
	meta := Metadata{
		Version:  DefaultVersion,
		Section:  DefaultSection,
		Priority: DefaultPriority,
		License:  DefaultLicense,
		OS:       DefaultOS,
		Maintainer: Maintainer{
			Name:  os.Getenv(EnvMaintainerName),
			Email: os.Getenv(EnvMaintainerMail),
		},
		Date: time.Now(),
	}
	d := fig.NewDecoder(r)
	d.Define(EnvArchive, kind)
	return meta, d.Decode(&meta)
}

func (m *Metadata) Update() error {
	for _, r := range m.Resources {
		m.Size += r.Size
	}
	return nil
}

func (m Metadata) PackageName() string {
	return fmt.Sprintf("%s-%s", m.Package, m.Version)
}

func (m *Metadata) HasChangelog() bool {
	return hasFile(m.Resources, Changelog)
}

func (m *Metadata) HasLicense() bool {
	return hasFile(m.Resources, License)
}

func hasFile(list []Resource, file string) bool {
	for _, r := range list {
		base := stripExt(r.File)
		if file == base {
			return true
		}
	}
	return false
}

func stripExt(file string) string {
	for {
		e := filepath.Ext(file)
		if e == "" {
			return file
		}
		file = strings.TrimSuffix(file, e)
	}
}

type Maintainer struct {
	Name  string
	Email string
}

func (m Maintainer) String() string {
	return m.Name
}

func (m Maintainer) IsZero() bool {
	return m.Name == ""
}

type Script struct {
	Code   string
	Digest string
}

// implements fig.Setter
func (s *Script) Set(str string) error {
	if str == "" {
		return nil
	}
	if b, err := os.ReadFile(str); err == nil {
		str = string(b)
	}
	if !strings.HasPrefix(str, "#!") {
		str = fmt.Sprintf("%s\n\n%s", DefaultShebang, str)
	}
	s.Code = str
	s.Digest = fmt.Sprintf("%x", md5.Sum([]byte(s.Code)))
	return nil
}

const (
	gzExt = ".gz"
)

type Resource struct {
	File     string
	Perm     int
	Archive  string `fig:"archive"`
	Compress bool
	Lang     string

	Inline  bool      `fig:"-"`
	Digest  string    `fig:"-"`
	Size    int64     `fig:"-"`
	ModTime time.Time `fig:"-"`
}

func (r Resource) Path() string {
	return r.Archive
}

// implements fig.Updater interface
func (r *Resource) Update() error {
	if r.Perm == 0 {
		r.Perm = 0644
	}

	f, err := os.Open(r.File)
	if err != nil {
		return err
	}
	defer f.Close()

	s, err := f.Stat()
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("%s is not a file", r.File)
	}

	var (
		sum           = md5.New()
		wrt io.Writer = sum
	)
	if r.Compress {
		wrt, _ = gzip.NewWriterLevel(wrt, gzip.BestCompression)
		if e := filepath.Ext(r.Archive); e != gzExt {
			r.Archive += gzExt
		}
	}
	r.Size, err = io.Copy(wrt, f)
	if c, ok := wrt.(io.Closer); ok {
		c.Close()
	}
	r.Digest = fmt.Sprintf("%x", sum.Sum(nil))
	r.ModTime = s.ModTime()

	return err
}

func (r Resource) IsConfig() bool {
	return strings.Contains(r.Archive, etcDir)
}

func (r Resource) IsDoc() bool {
	return strings.Contains(r.Archive, docDir)
}

func (r Resource) IsRegular() bool {
	return false
}

func (r Resource) IsLicense() bool {
	return filepath.Base(r.File) == License || filepath.Base(r.Archive) == License
}

func (r Resource) IsReadme() bool {
	return filepath.Base(r.File) == Readme || filepath.Base(r.Archive) == Readme
}

type Change struct {
	Title      string
	Desc       string `fig:"description"`
	Version    string
	When       time.Time
	Maintainer Maintainer
}
