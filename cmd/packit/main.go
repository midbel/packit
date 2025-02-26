package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/midbel/packit/internal/deb"
	"github.com/midbel/packit/internal/packfile"
)

func main() {
	var (
		kind = flag.String("k", "", "package type")
		file = flag.String("f", "", "package file")
		dist = flag.String("d", "", "directory")
	)
	flag.Parse()

	pkg, err := packfile.Load(*file, flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	name := fmt.Sprintf("%s.%s", pkg.PackageName(), *kind)
	name = filepath.Join(*dist, name)
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Remove(name)
	w, err := os.Create(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer w.Close()

	builder, err := deb.Build(w)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if c, ok := builder.(io.Closer); ok {
		defer c.Close()
	}
	if err := builder.Build(pkg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
