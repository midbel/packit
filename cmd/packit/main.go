package main

import (
	"fmt"
	"flag"
	"os"

	"github.com/midbel/packit/internal/packfile"
)

func main() {
	kind := flag.String("k", "", "package type")
	flag.Parse()

	pkg, err := packfile.Load(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: %+s\n", kind, pkg)
}
