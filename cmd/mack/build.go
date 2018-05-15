package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/midbel/cli"
	"github.com/midbel/mack"
	"github.com/midbel/mack/deb"
	"github.com/midbel/mack/rpm"
	"github.com/midbel/toml"
	"golang.org/x/sync/errgroup"
)

func runBuild(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var g errgroup.Group
	for _, a := range cmd.Flag.Args() {
		file := a
		g.Go(func() error {
			return makePackage(file)
		})
	}
	return g.Wait()
}

func makePackage(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	c := struct {
		Location string       `toml:"location"`
		Type     string       `toml:"type"`
		Control  mack.Control `toml:"control"`
		Files    []*mack.File `toml:"resource"`
	}{}
	c.Control.Maintainer = mack.Maintainer{
		Name:  os.Getenv("MACK_MAINTAINER"),
		Email: os.Getenv("MACK_EMAIL"),
	}
	if err := toml.NewDecoder(f).Decode(&c); err != nil {
		return err
	}
	if c.Type != "rpm" && c.Type != "deb" {
		return fmt.Errorf("package type not recognized: %q", c.Type)
	}
	if err := os.MkdirAll(c.Location, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if c.Control.Version == "" {
		c.Control.Version = "0.0"
	}
	n := fmt.Sprintf("%s-%s.%s", c.Control.Package, c.Control.Version, c.Type)
	w, err := os.Create(filepath.Join(c.Location, n))
	if err != nil {
		return err
	}
	defer w.Close()

	var pkg mack.Builder
	switch c.Type {
	case "rpm":
		pkg = rpm.NewBuilder(w)
	case "deb":
		pkg, err = deb.NewBuilder(w)
	}
	if err != nil {
		return err
	}
	if err := pkg.Build(c.Control, c.Files); err != nil {
		os.Remove(c.Location)
		return err
	}
	return nil
}
