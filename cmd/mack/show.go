package main

import (
	"fmt"

	"github.com/midbel/cli"
	// "github.com/midbel/mack"
	"golang.org/x/sync/errgroup"
)

func runShow(cmd *cli.Command, args []string) error {
	check := cmd.Flag.Bool("c", false, "check")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var g errgroup.Group
	for _, a := range cmd.Flag.Args() {
		a := a
		g.Go(func() error {
			return showDEB(a, *check)
		})
	}
	return g.Wait()
}

func showDEB(file string, check bool) error {
	return fmt.Errorf("not yet implemented")
}
