package main

import (
	"fmt"
	"os"

	"github.com/midbel/cli"
	"github.com/midbel/mack"
	"github.com/midbel/mack/deb"
	"github.com/midbel/toml"
	"golang.org/x/sync/errgroup"
)

func runDeb(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var g errgroup.Group
	for _, a := range cmd.Flag.Args() {
		file := a
		g.Go(func() error {
			return makeDEB(file)
		})
	}
	return g.Wait()
}

func makeDEB(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	c := struct {
		Location string       `toml:"location"`
		Control  deb.Control  `toml:"control"`
		Files    []*mack.File `toml:"resource"`
	}{}
	c.Control.Maintainer = mack.Maintainer{
		Name:  os.Getenv("MACK_MAINTAINER"),
		Email: os.Getenv("MACK_EMAIL"),
	}
	if err := toml.NewDecoder(f).Decode(&c); err != nil {
		return err
	}
	d, err := os.Create(c.Location)
	if err != nil {
		return err
	}
	defer d.Close()

	pkg, err := deb.NewWriter(d)
	if err != nil {
		return err
	}
	if err := pkg.WriteControl(c.Control); err != nil {
		return fmt.Errorf("failed to write control:", err)
	}
	for _, f := range c.Files {
		if err := pkg.WriteFile(f); err != nil {
			return fmt.Errorf("failed to write file:", f.Dst, err)
		}
	}
	err = pkg.Close()
	if err != nil {
		os.Remove(c.Location)
	}
	return err
}
