package packfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Maintainer struct {
	Name  string
	Email string
}

type Dependency struct {
	Package    string
	Constraint string // gt, ge, lt, le,...
	Version    string
	Arch       string
	Type       string // breaks, suggests, recommands,...
}

type Change struct {
	Summary string
	Changes []string
	Version string
	When    time.Time
	Maintainer
}

type Resource struct {
	Path string
	Local    io.ReadCloser
	Target   string
	Perm     int64
	Compress bool

	Flags int64

	Size    int64
	Lastmod time.Time
	Hash    string
}

func (r Resource) IsConfig() bool {
	return r.Flags&FileFlagConf == FileFlagConf
}

func (r Resource) IsRegular() bool {
	return r.Flags&FileFlagRegular != 0
}

func (r Resource) IsDirectory() bool {
	return r.Flags&FileFlagDir == FileFlagDir
}

type Compiler struct {
	Name    string
	Version string
}

type Package struct {
	Setup    string
	Teardown string

	Name    string
	Summary string
	Desc    string
	Version string
	Release string
	Home    string
	Vendor  string
	Distrib string

	BuildWith   Compiler
	PackageType string

	Essential bool

	Arch     string
	Os       string
	Section  string
	Priority string

	License string

	PreInst     string
	PostInst    string
	PreRem      string
	PostRem     string
	CheckScript string

	Maintainer Maintainer
	Depends    []Dependency
	Changes    []Change

	Digest int
	Files  []Resource
}

func Load(file, context string) (*Package, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	d, err := NewDecoder(r, context)
	if err != nil {
		return nil, err
	}
	return d.Decode()
}

func (p *Package) GetDirDoc(file string) string {
	return filepath.Join(DirDoc, p.Name, file)
}

func (p *Package) Requires() []Dependency {
	return p.depends("depends")
}

func (p *Package) Recommends() []Dependency {
	return p.depends("recommends")
}

func (p *Package) Suggests() []Dependency {
	return p.depends("suggests")
}

func (p *Package) Breaks() []Dependency {
	return p.depends("breaks")
}

func (p *Package) Conflicts() []Dependency {
	return p.depends("conflicts")
}

func (p *Package) Replaces() []Dependency {
	return p.depends("replaces")
}

func (p *Package) Enhances() []Dependency {
	return p.depends("enhances")
}

func (p *Package) Provides() []Dependency {
	return p.depends("provides")
}

func (p *Package) depends(kind string) []Dependency {
	var list []Dependency
	for _, d := range p.Depends {
		if d.Type == kind {
			list = append(list, d)
		}
	}
	return list
}

func (p *Package) TotalSize() int64 {
	var z int64
	for _, r := range p.Files {
		z += r.Size
	}
	return z
}

func (p *Package) PackageName() string {
	return fmt.Sprintf("%s-%s", p.Name, p.Version)
}
