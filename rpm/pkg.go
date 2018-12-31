package rpm

import (
	"github.com/midbel/packit"
)

type rpm struct {
}

func (r *rpm) PackageName() string {
	return "packit.rpm"
}

func (r *rpm) Valid() error {
	return nil
}

func (r *rpm) About() packit.Control {
	var c packit.Control
	return c
}

func (r *rpm) Filenames() ([]string, error) {
	return nil, nil
}
