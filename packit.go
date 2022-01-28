package packit

import (
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	return filepath.Join(r.Dir, filepath.Base(r.File))
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
		sum = md5.New()
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

type FileInfo struct {
	File string

	Inline  bool      `fig:"-"`
	Digest  string    `fig:"-"`
	Size    int64     `fig:"-"`
	ModTime time.Time `fig:"-"`

	Sub []Resource
}

// implements fig.Setter interface
func (f *FileInfo) Set(str string) error {
	f.File = str
	r, err := os.Open(f.File)
	if err != nil {
		return err
	}
	defer r.Close()

	s, err := r.Stat()
	if err != nil {
		return err
	}
	if s.IsDir() {
		return f.sub(f.File)
	}

	var (
		sum = md5.New()
		wrt io.Writer = sum
	)
	// Resource has Compress option but not FileInfo
	// if f.Compress {
	// 	wrt, _ = gzip.NewWriterLevel(sum, gzip.BestCompression)
	// }
	f.Size, err = io.Copy(sum, r)
	if c, ok := wrt.(io.Closer); ok {
		c.Close()
	}
	f.Digest = fmt.Sprintf("%x", sum.Sum(nil))
	f.ModTime = s.ModTime()

	return err
}

func (f *FileInfo) sub(dir string) error {
	return nil
}

type Change struct {
	Title      string
	Desc       string `fig:"description"`
	Version    string
	When       time.Time
	Maintainer Maintainer
}
