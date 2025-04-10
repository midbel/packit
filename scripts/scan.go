package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/packit/internal/packfile"
)

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer r.Close()

	scan := packfile.Scan(r)
	for {
		tok := scan.Scan()
		fmt.Println(tok)
		if tok.Type == packfile.EOF {
			break
		}
	}
}
