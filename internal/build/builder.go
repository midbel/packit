package build

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/midbel/packit/internal/deb"
	"github.com/midbel/packit/internal/packfile"
	"github.com/midbel/packit/internal/rpm"
	"github.com/midbel/tape"
)

type Builder interface {
	Build(*packfile.Package) error
}

//go:embed templates/rpm_info.txt
var rpmInfoFile string

//go:embed templates/deb_info.txt
var debInfoFile string

func Info(file string, w io.Writer) error {
	var (
		pkg  any
		err  error
		info string
	)
	switch ext := filepath.Ext(file); ext {
	case ".deb":
		info = debInfoFile
		pkg, err = deb.Info(file)
	case ".rpm":
		info = rpmInfoFile
		pkg, err = rpm.Info(file)
	default:
		return fmt.Errorf("%s: package type not supported", ext)
	}
	if err != nil {
		return err
	}
	tpl, err := template.New("info").Parse(info)
	if err != nil {
		return err
	}
	return tpl.Execute(w, pkg)
}

func Content(file string, w io.Writer) error {
	var (
		list []*tape.Header
		err  error
	)
	switch ext := filepath.Ext(file); ext {
	case ".deb":
		list, err = deb.Content(file)
	case ".rpm":
	default:
		return fmt.Errorf("%s: package type not supported", ext)
	}
	if err != nil || len(list) == 0 {
		return err
	}
	for _, h := range list {
		var (
			owner string
			group string
			when  = h.ModTime.Format("Jan 02 15:04")
		)
		if u, err := user.LookupId(strconv.Itoa(int(h.Uid))); err == nil {
			owner = u.Username
		}
		if g, err := user.LookupGroupId(strconv.Itoa(int(h.Gid))); err == nil {
			group = g.Name
		}
		fmt.Fprintf(w, "%s %-8s %-8s %8d %s %s", os.FileMode(h.Mode), owner, group, h.Size, when, h.Filename)
		fmt.Fprintln(w)
	}
	return nil
}

func CheckPackage(file string) error {
	switch ext := filepath.Ext(file); ext {
	case ".deb":
		return deb.Check(file)
	case ".rpm":
		return rpm.Check(file)
	default:
		return fmt.Errorf("%s: package type not supported", ext)
	}
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
		return nil, fmt.Errorf("%s: package type not supported", kind)
	}
}
