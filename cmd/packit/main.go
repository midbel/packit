package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/packit"
	"github.com/midbel/packit/deb"
	"github.com/midbel/packit/rpm"
)

func main() {
	var (
		dir  = flag.String("d", "", "output directory")
		kind = flag.String("k", "", "package type")
	)
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	m, err := packit.Load(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	switch *kind {
	case packit.DEB, "":
		err = deb.Build(*dir, m)
	case packit.RPM:
		err = rpm.Build(*dir, m)
	default:
		err = fmt.Errorf("%s: unsupported package type", *kind)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
