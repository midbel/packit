package main

import (
	"flag"
	"os"

	"github.com/midbel/packit/rpm"
)

func main() {
	flag.Parse()
	for _, p := range flag.Args() {
		if err := rpm.Debug(p, os.Stdout); err != nil {
			os.Exit(1)
		}
	}
}
