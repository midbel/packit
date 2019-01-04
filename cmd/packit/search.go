package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/midbel/cli"
	"github.com/midbel/packit"
	"github.com/midbel/packit/deb/control"
)

const (
	dpkgBase = "/var/lib/dpkg"
)

func runSearch(cmd *cli.Command, args []string) error {
	arch := cmd.Flag.Int("a", -1, "architecture")
	kind := cmd.Flag.String("k", "", "package type")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	switch *kind {
	case "", "rpm":
		return fmt.Errorf("%s not yet supported", *kind)
	case "deb", "dpkg":
		return searchDebPackage(cmd.Flag.Arg(0), *arch)
	}
	return nil
}

func searchDebPackage(n string, a int) error {
	r, err := os.Open(filepath.Join(dpkgBase, "status"))
	if err != nil {
		return err
	}
	defer r.Close()

	cs, err := control.ParseMulti(r)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
	defer w.Flush()
	for _, c := range cs {
		if !(strings.Contains(c.Package, n) || strings.Contains(c.Source, n)) {
			continue
		}
		if a >= 0 && c.Arch != uint8(a) {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Package, c.Version, packit.ArchString(c.Arch), c.Summary)
	}
	return nil
}
