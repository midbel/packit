package packit

import (
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
	DEB = "deb"
	RPM = "rpm"
)

const (
	Arch64 = 64
	Arch32 = 32
)

const (
	DefaultVersion  = "0.1.0"
	DefaultLicense  = "gpl-3.0"
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

type Metadata struct {
	Package  string
	Version  string
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

func Load(r io.Reader) (Metadata, error) {
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
	return meta, fig.NewDecoder(r).Decode(&meta)
}

func (m Metadata) GetLicense(dir string) (Resource, error) {
	res, ok := findLicense(m.Resources)
	if ok {
		return res, nil
	}
	var (
		name   = fmt.Sprintf("%s.tpl", m.License)
		b, err = fs.ReadFile(licenses, filepath.Join("licenses", name))
	)
	if err != nil {
		return res, err
	}
	tpl, err := template.New("license").Parse(string(b))
	if err != nil {
		return res, err
	}
	ctx := struct {
		Year   int
		Holder string
	}{
		Year:   m.Date.Year(),
		Holder: m.Maintainer.Name,
	}
	if err := tpl.Execute(os.Stdout, ctx); err != nil {
		return res, err
	}
	res.File = "LICENSE"
	res.Perm = 0644
	res.Dir  = filepath.Join(dir, m.Package)
	// res.Size =
	// res.Digest =

	return res, nil
}

func (m *Metadata) Update() error {
	for _, r := range m.Resources {
		m.Size += r.Size
	}
	return nil
}

type Maintainer struct {
	Name  string
	Email string
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

const fileCopyright = "copyright"

type Resource struct {
	File     string
	Perm     int    `fig:"permission"`
	Dir      string `fig:"directory"`
	Compress bool
	Lang     string

	Inline  bool      `fig:"-"`
	Digest  string    `fig:"-"`
	Size    int64     `fig:"-"`
	ModTime time.Time `fig:"-"`
}

func (r Resource) Path() string {
	switch base := filepath.Base(r.Dir); base {
	case fileCopyright:
		return r.Dir
	default:
	}
	if r.Compress {
		if filepath.Ext(r.File) != ".gz" {
			r.File += ".gz"
		}
	}
	return r.Dir
}

func (r Resource) IsConfig() bool {
	return false
}

func (r Resource) IsDoc() bool {
	return false
}

// implements fig.Updater interface
func (r *Resource) Update() error {
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
	}
	r.Size, err = io.Copy(wrt, f)
	if c, ok := wrt.(io.Closer); ok {
		c.Close()
	}
	r.Digest = fmt.Sprintf("%x", sum.Sum(nil))
	r.ModTime = s.ModTime()

	return err
}

type Change struct {
	Title      string
	Desc       string `fig:"description"`
	Version    string
	When       time.Time
	Maintainer Maintainer
}

func findLicense(res []Resource) (Resource, bool) {
	for _, r := range res {
		base := filepath.Base(r.Dir)
		switch base := strings.ToLower(base); {
		case base == "copyright" || base == "license":
			return r, true
		case r.File == "license" || r.File == "LICENSE":
			return r, true
		default:
		}
	}
	return Resource{}, false
}
