package deb

import (
	"bytes"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/cedar/ar"
)

const (
	DebVersion       = "2.0\n"
	DebDataFile      = "data.tar.gz"
	DebControlTar    = "control.tar.gz"
	DebBinaryFile    = "debian-binary"
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

type Writer struct {
	inner   *ar.Writer
	modtime time.Time
}

func NewWriter(w io.Writer) (*Writer, error) {
	n := time.Now()
	aw := ar.NewWriter(w)
	if err := writeDebianBinaryFile(aw, n); err != nil {
		return nil, err
	}
	return &Writer{aw, n}, nil
}

func writeDebianBinaryFile(a *ar.Writer, n time.Time) error {
	h := ar.Header{
		Name:    DebBinaryFile,
		Uid:     0,
		Gid:     0,
		ModTime: n,
		Mode:    0644,
		Length:  len(DebVersion),
	}
	if err := a.WriteHeader(&h); err != nil {
		return err
	}
	if _, err := io.WriteString(a, DebVersion); err != nil {
		return err
	}
	return nil
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
