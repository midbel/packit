package build

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func Info(file string, all, deps bool, w io.Writer) error {
	if deps && !all {
		return getPackageDeps(file, w)
	}
	if err := getPackageInfos(file, w); err != nil {
		return err
	}
	if all {
		return getPackageDeps(file, w)
	}
	return nil
}

func getPackageDeps(file string, w io.Writer) error {
	var (
		list []string
		err  error
	)
	switch ext := filepath.Ext(file); ext {
	case ".deb":
		list, err = deb.Dependencies(file)
	case ".rpm":
		list, err = rpm.Dependencies(file)
	default:
		return fmt.Errorf("%s: package type not supported", ext)
	}
	if err != nil || len(list) == 0 {
		return err
	}
	fmt.Fprintln(w, "Dependencies:")
	for _, d := range list {
		fmt.Fprintln(w, "- "+d)
	}
	return nil
}

func getPackageInfos(file string, w io.Writer) error {
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
		list, err = rpm.Content(file)
	default:
		return fmt.Errorf("%s: package type not supported", ext)
	}
	if err != nil || len(list) == 0 {
		return err
	}
	for _, h := range list {
		when := h.ModTime.Format("Jan 02 15:04")
		fmt.Fprintf(w, "%s %-8s %-8s %8d %s %s", os.FileMode(h.Mode), h.User(), h.Group(), h.Size, when, h.Filename)
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

type PackageBuilder struct {
	File      string
	Dist      string
	Type      string
	OnlyDocs  bool
	SplitDocs bool
}

func (b *PackageBuilder) BuildPackage(context string) error {
	if context == "" {
		context = filepath.Dir(b.File)
	}
	pkg, err := packfile.Load(b.File, context)
	if err != nil {
		return err
	}

	var all []*packfile.Package
	if b.OnlyDocs {
		all = append(all, pkg.OnlyDocs())
	} else if b.SplitDocs {
		all = pkg.Split()
	} else {
		all = append(all, pkg)
	}

	for _, pkg := range all {
		if err := b.buildPackage(pkg); err != nil {
			return err
		}
	}
	return nil
}

func (b *PackageBuilder) buildPackage(pkg *packfile.Package) error {
	name := fmt.Sprintf("%s.%s", pkg.PackageName(), b.Type)
	name = filepath.Join(b.Dist, name)
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	w, err := os.Create(name)
	if err != nil {
		return err
	}
	defer w.Close()

	builder, err := Build(b.Type, w)
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
