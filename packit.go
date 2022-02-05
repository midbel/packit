package packit

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"embed"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/fig"
)

var ErrPackage = errors.New("unsupported package type")

const (
	EnvArchive = "archive"
	EnvBash    = "bash"
	EnvShell   = "shell"
	EnvPwsh    = "pwsh"
	EnvPython  = "python"
	EnvPerl    = "perl"

	envHash = "hash"
)

const (
	DEB = "deb"
	RPM = "rpm"
	APK = "apk"
)

const (
	docDir = "doc"
	etcDir = "etc"
)

const (
	Root       = "root"
	Shebang    = "#!"
	Bash       = "/bin/bash"
	Shell      = "/bin/sh"
	Powershell = "/usr/bin/pwsh"
	Python     = "/usr/bin/env python3"
	Perl       = "/usr/bin/perl"
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

func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		h = "localhost"
	}
	return h
}

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

	Essential  bool
	Depends    []Dependency `fig:"depend"`
	Suggests   []Dependency `fig:"suggest"`
	Provides   []Dependency `fig:"provide"`
	Breaks     []Dependency `fig:"break"`
	Conflicts  []Dependency `fig:"conflict"`
	Replaces   []Dependency `fig:"replace"`
	Requires   []Dependency `fig:"require"`
	Recommands []Dependency `fig:"recommand"`
	Obsoletes  []Dependency `fig:"obsolet"`

	PreInst  Script `fig:"pre-install"`
	PostInst Script `fig:"post-install"`
	PreRem   Script `fig:"pre-remove"`
	PostRem  Script `fig:"post-remove"`

	Date time.Time `fig:"-"`
	Size int64     `fig:"-"`
}

var fmap = fig.FuncMap{
	"dirname":   filepath.Dir,
	"basename":  filepath.Base,
	"cleanpath": filepath.Clean,
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
		Date: time.Now().UTC(),
	}
	d := fig.NewDecoder(r)
	d.Funcs(fmap)

	d.Define(EnvArchive, kind)
	d.Define(EnvBash, Bash)
	d.Define(EnvShell, Shell)
	d.Define(EnvPwsh, Powershell)
	d.Define(EnvPython, Python)
	d.Define(EnvPerl, Perl)
	return meta, d.Decode(&meta)
}

func (m *Metadata) Update(_ fig.Resolver) error {
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

type Condition int

const (
	Eq Condition = 1 << iota
	Lt
	Le
	Gt
	Ge
)

type Dependency struct {
	Name    string
	Version string
	Cond    Condition
}

func ParseDependency(str string) (Dependency, error) {
	return parseDependency(str)
}

func (d Dependency) String() string {
	if d.Cond == 0 {
		return d.Name
	}
	var op string
	switch d.Cond {
	case Eq:
		op = "="
	case Gt:
		op = ">"
	case Ge:
		op = ">="
	case Lt:
		op = "<"
	case Le:
		op = ">="
	}
	return fmt.Sprintf("%s %s %s", d.Name, op, d.Version)
}

func (d *Dependency) Set(str string) error {
	x, err := ParseDependency(str)
	if err == nil {
		*d = x
	}
	return err
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
	Program string
	Code    string `fig:"script"`
	Digest  string
}

// implements fig.Updater
func (s *Script) Update(_ fig.Resolver) error {
	if s.Code == "" {
		return nil
	}
	if b, err := os.ReadFile(s.Code); err == nil {
		s.Code = string(b)
	}
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
func (r *Resource) Update(res fig.Resolver) error {
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
		sum hash.Hash
		wrt io.Writer
	)
	if sum, err = getHash(res); err != nil {
		return err
	}
	wrt = sum

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

func getHash(res fig.Resolver) (hash.Hash, error) {
	v, err := res.Resolve(envHash)
	if err != nil {
		return md5.New(), nil
	}
	var (
		str, _ = v.(string)
		hasher hash.Hash
	)
	switch str {
	case "", "md5":
		hasher = md5.New()
	case "sha1":
		hasher = sha1.New()
	case "sha256":
		hasher = sha256.New()
	case "sha512":
		hasher = sha512.New()
	default:
		return nil, fmt.Errorf("%s: unsupported hash", str)
	}
	return hasher, nil
}

type Change struct {
	Title      string
	Desc       string `fig:"description"`
	Version    string
	When       time.Time
	Maintainer Maintainer
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

func parseDependency(str string) (Dependency, error) {
	data := []struct {
		Op string
		Cd Condition
	}{
		{Op: "<=", Cd: Le},
		{Op: ">=", Cd: Ge},
		{Op: "<", Cd: Lt},
		{Op: ">", Cd: Gt},
		{Op: "=", Cd: Eq},
	}
	var dep Dependency
	for _, d := range data {
		x := strings.Index(str, d.Op)
		if x > 0 {
			parts := strings.Split(str, d.Op)
			if len(parts) != 2 {
				return dep, fmt.Errorf("%s: syntax error (dependency)")
			}
			dep.Name = strings.TrimSpace(parts[0])
			dep.Version = strings.TrimSpace(parts[1])
			dep.Cond = d.Cd
			return dep, nil
		}
	}
	dep.Name = str
	return dep, nil
}
