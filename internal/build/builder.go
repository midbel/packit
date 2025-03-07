package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/midbel/packit/internal/deb"
	"github.com/midbel/packit/internal/packfile"
	"github.com/midbel/packit/internal/rpm"
)

type Builder interface {
	Build(*packfile.Package) error
}

func BuildPackage(file, dist, kind, context string) error {
	if context == "" {
		context = filepath.Dir(file)
	}
	pkg, err := packfile.Load(file, context)
	if err != nil {
		return err
	}

	name := fmt.Sprintf("%s.%s", pkg.PackageName(), kind)
	name = filepath.Join(dist, name)
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	w, err := os.Create(name)
	if err != nil {
		return err
	}
	defer w.Close()

	builder, err := Build(kind, w)
	if err != nil {
		return err
	}
	if c, ok := builder.(io.Closer); ok {
		defer c.Close()
	}
	return builder.Build(pkg)
}

func Build(kind string, w io.Writer) (Builder, error) {
	switch kind {
	case packfile.Deb:
		return deb.Build(w)
	case packfile.Rpm:
		return rpm.Build(w)
	default:
		return nil, fmt.Errorf("%s: package type not supported")
	}
}
