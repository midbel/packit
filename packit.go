package packit

import (
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
)

const (
	EnvMaintainerName = "PACKIT_MAINTAINER_NAME"
	EnvMaintainerMail = "PACKIT_MAINTAINER_MAIL"
)

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

	Depends   []string `fig:"depend"`
	Suggests  []string `fig:"suggest"`
	Provides  []string `fig:"provide"`
	Breaks    []string `fig:"break"`
	Conflicts []string `fig:"conflict"`
	Replaces  []string `fig:"replace"`

	PreInst  string `fig:"pre-install"`
	PostInst string `fig:"post-install"`
	PreRem   string `fig:"pre-remove"`
	PostRem  string `fig:"post-remove"`

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

func (m *Metadata) Update() error {
	read := func(res Resource) (Resource, error) {
		r, err := os.Open(res.File)
		if err != nil {
			return res, err
		}
		defer r.Close()

		var (
			sum           = md5.New()
			wrt io.Writer = sum
		)
		if res.Compress {
			wrt, _ = gzip.NewWriterLevel(wrt, gzip.BestCompression)
		}
		res.Size, err = io.Copy(wrt, r)
		if c, ok := wrt.(io.Closer); ok {
			c.Close()
		}
		res.Digest = fmt.Sprintf("%x", sum.Sum(nil))
		res.ModTime = time.Now()
		return res, err
	}
	for i := range m.Resources {
		res, err := read(m.Resources[i])
		if err != nil {
			return err
		}
		m.Resources[i] = res
		m.Size += res.Size
	}
	return nil
}

type Maintainer struct {
	Name  string
	Email string
}

type Script struct {
	Content string
}

// implements fig.Setter
func (s *Script) Set(v interface{}) error {
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

// implements fig.Setter interface
func (r *Resource) Set(v interface{}) error {
	return nil
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
	return filepath.Join(r.Dir, filepath.Base(r.File))
}

func (r Resource) IsConfig() bool {
	return false
}

func (r Resource) IsDoc() bool {
	return false
}

type Change struct {
	Title      string
	Desc       string `fig:"description"`
	Version    string
	When       time.Time
	Maintainer Maintainer
}
