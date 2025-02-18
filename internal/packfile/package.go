package packfile

import (
	"embed"
	"fmt"
	"os"
	"time"
)

const (
	Deb = "deb"
	Rpm = "rpm"
	Apk = "apk"
)

const (
	Changelog = "CHANGELOG"
	License   = "LICENSE"
	Readme    = "README"
)

const (
	Arch64 = "amd64"
	Arch32 = "i386"
)

const (
	EnvMaintainerName = "PACK_MAINTAINER_NAME"
	EnvMaintainerMail = "PACK_MAINTAINER_MAIL"
)

const (
	EnvArchive = "archive"
	EnvBash    = "bash"
	EnvShell   = "shell"

	envHash = "hash"
)

const (
	DefaultVersion  = "0.1.0"
	DefaultLicense  = "mit"
	DefaultSection  = "contrib"
	DefaultPriority = "extra"
	DefaultOS       = "linux"
	DefaultShell    = "/bin/sh"
)

const (
	constraintEq = "eq"
	constraintNe = "ne"
	constraintGt = "gt"
	constraintGe = "ge"
	constraintLt = "lt"
	constraintLe = "le"
)

func getVersionContraint(constraint string) (string, error) {
	switch constraint {
	case constraintEq:
		constraint = "="
	case constraintNe:
		constraint = "!="
	case constraintGt:
		constraint = ">"
	case constraintGe:
		constraint = ">="
	case constraintLt:
		constraint = "<"
	case constraintLe:
		constraint = "<="
	default:
		return "", fmt.Errorf("%s: invalid constraint given", constraint)
	}
	return constraint, nil
}

//go:embed licenses/*
var licenseFiles embed.FS

type Maintainer struct {
	Name  string
	Email string
}

type Dependency struct {
	Package    string
	Constraint string // gt, ge, lt, le,...
	Version    string
	Type       string // breaks, suggests, recommands,...
	Arch       string
}

type Change struct {
	Summary string
	Desc    string
	Version string
	When    time.Time
	Maintainer
}

type Resource struct {
	Local  *os.File
	Target string
	Perm   int64

	size    int64
	lastmod time.Time
}

type Package struct {
	Name    string
	Summary string
	Desc    string
	Version string
	Release string

	Compiler    string
	PackageType string

	Essential bool

	Arch     string
	Os       string
	Section  string
	Priority string

	License string

	PreInst  string
	PostInst string
	PreRem   string
	PostRem  string

	Maintainers []Maintainer
	Depends     []Dependency
	Changes     []Change

	Files []Resource
}

func Load(file string) (*Package, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	d, err := NewDecoder(r)
	if err != nil {
		return nil, err
	}
	return d.Decode()
}

func (p *Package) PackageName() string {
	return fmt.Sprintf("%s-%s", p.Name, p.Version)
}

type Environ struct {
	parent *Environ
	values map[string]any
}

func Empty() *Environ {
	e := Environ{
		values: make(map[string]any),
	}
	return &e
}

func (e *Environ) Define(ident string, value any) error {
	_, ok := e.values[ident]
	if ok {
		return fmt.Errorf("identifier %q already defined", ident)
	}
	e.values[ident] = value
	return nil
}

func (e *Environ) Resolve(ident string) (any, error) {
	v, ok := e.values[ident]
	if ok {
		return v, nil
	}
	if e.parent != nil {
		return e.parent.Resolve(ident)
	}
	return nil, fmt.Errorf("undefined variable %s", ident)
}
