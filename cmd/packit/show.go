package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/midbel/cli"
	"github.com/midbel/packit"
	"github.com/midbel/packit/deb"
	"github.com/midbel/packit/rpm"
)

func runShow(cmd *cli.Command, args []string) error {
	long := cmd.Flag.Bool("l", false, "show full package description")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	if args := cmd.Flag.Args(); *long {
		return showDescription(args)
	} else {
		return showAvailable(args)
	}
}

func showAvailable(ns []string) error {
	w := tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
	defer w.Flush()
	return showPackages(ns, func(p packit.Package) error {
		c := p.About()
		fmt.Fprintf(w, "%s\t%s\n", p.PackageName(), c.Summary)
		return nil
	})
}

func showDescription(ns []string) error {
	return nil
}

func showPackages(ns []string, fn func(packit.Package) error) error {
	if fn == nil {
		return nil
	}
	for _, n := range ns {
		var (
			pkg packit.Package
			err error
		)
		switch e := filepath.Ext(n); e {
		case ".deb":
			pkg, err = deb.Open(n)
		case ".rpm":
			pkg, err = rpm.Open(n)
		default:
			return fmt.Errorf("unsupported packet type %s", e)
		}
		if err != nil {
			return err
		}
		if err := fn(pkg); err != nil {
			return err
		}
	}
	return nil
}
