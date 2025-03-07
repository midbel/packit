package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/packit/internal/build"
)

func main() {
	var (
		kind = flag.String("k", "", "package type")
		file = flag.String("f", "Packfile", "package file")
		dist = flag.String("d", "", "directory where package will be written")
	)
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "no context given")
		os.Exit(2)
	}

	err := build.BuildPackage(*file, *dist, *kind, flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
