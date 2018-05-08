package mack

import (
	"fmt"
	"path/filepath"
	"strings"
)

const etcDir = "etc/"

type Builder interface {
	Build(Control, []*File) error
}

type Maintainer struct {
	Name  string `toml:"name"`
	Email string `toml:"email"`
}

func (m Maintainer) String() string {
	return fmt.Sprintf("%s <%s>", m.Name, m.Email)
}

type Control struct {
	Package      string       `toml:"package"`
	Version      string       `toml:"version"`
	Release      string       `toml:"release"`
	Summary      string       `toml:"summary"`
	Desc         string       `toml:"description"`
	License      string       `toml:"license"`
	Section      string       `toml:"section"`
	Priority     string       `toml:"priority"`
	Os           string       `toml:"os"`
	Arch         string       `toml:"arch"`
	Vendor       string       `toml:"vendor"`
	Home         string       `toml:"homepage"`
	Depends      []string     `toml:"depends"`
	Compiler     string       `toml:"compiler"`
	Size         int          `toml:"size"`
	Contributors []Maintainer `toml:"contributors"`
	Maintainer   `toml:"maintainer"`
}

type File struct {
	Src      string `toml:"source"`
	Dst      string `toml:"destination"`
	Name     string `toml:"filename"`
	Compress bool   `toml:"compress"`
	Perm     int    `toml:"mode"`
	Conf     bool   `toml:"conf"`
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
	return strings.Index(n, etcDir) >= 0
}
