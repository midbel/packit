package deb

import (
	"bytes"
	"strings"
	"text/template"
)

const (
	DebVersion       = "2.0\n"
	DebDataFile      = "data.tar.gz"
	DebControlTar    = "control.tar.gz"
	DebBinaryTar     = "debian-binary"
	DebControlFile   = "./control"
	DebMD5sumsFile   = "./md5sums"
	DebConffilesFile = "./conffiles"
)

const control = `
Package: {{.Package}}
Version: {{.Version}}
License: {{.License}}
Section: {{.Section}}
Priority: {{.Priority}}
Architecture: {{.Arch}}
Vendor: {{.Vendor}}
Maintainer: {{.Name}} <{{.Email}}>
Pre-Depends: {{join .Depends ", "}}
Installed-Size: {{.Size}}
Build-Using: {{.Compiler}}
Description: {{.Summary}}
`

type Maintainer struct {
	Name  string `toml:"name"`
	Email string `toml:"email"`
}

type Control struct {
	Package    string   `toml:"package"`
	Version    string   `toml:"version"`
	Summary    string   `toml:"summary"`
	License    string   `toml:"license"`
	Section    string   `toml:"section"`
	Priority   string   `toml:"priority"`
	Arch       string   `toml:"arch"`
	Vendor     string   `tom:"vendor"`
	Depends    []string `toml:"depends"`
	Conffiles  []string `toml:"conffiles"`
	Compiler   string   `toml:"compiler"`
	Size       int      `toml:"size"`
	Maintainer `toml:"maintainer"`
}

func prepareControl(c Control) (*bytes.Buffer, error) {
	fs := template.FuncMap{
		"join": strings.Join,
	}
	t, err := template.New("control").Funcs(fs).Parse(strings.TrimSpace(control) + "\n")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, c); err != nil {
		return nil, err
	}
	return &buf, nil
}
