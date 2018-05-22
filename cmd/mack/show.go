package main

import (
	"fmt"
	"path/filepath"

	"github.com/midbel/cli"
	"github.com/midbel/mack/deb"
)

func runShow(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var show func(string) error
	switch e := filepath.Ext(cmd.Flag.Arg(0)); e {
	case ".deb":
		show = showDEB
	case ".rpm":
		show = showRPM
	default:
		return fmt.Errorf("unknown packet type")
	}
	return show(cmd.Flag.Arg(0))
}

func showDEB(file string) error {
	pkg, err := deb.Open(file)
	if err != nil {
		return err
	}
	c, err := pkg.Control()
	if err != nil {
		return err
	}
	fmt.Printf("%-12s: %s\n", "Package", c.Package)
	fmt.Printf("%-12s: %s\n", "Version", c.Version)
	fmt.Printf("%-12s: %s\n", "License", c.License)
	fmt.Printf("%-12s: %s\n", "Section", c.Section)
	fmt.Printf("%-12s: %s\n", "Priority", c.Priority)
	fmt.Printf("%-12s: %.2fKB\n", "Size", float64(c.Size)/1024)
	fmt.Printf("%-12s: %s\n", "Maintainer", c.Maintainer)
	fmt.Println()
	fmt.Println(c.Summary)
	fmt.Println(c.Desc)
	fmt.Println()
	fmt.Printf("%-12s: \n", "Recommends")
	fmt.Printf("%-12s: \n", "Depends")
	fmt.Printf("%-12s: \n", "Breaks")
	fmt.Printf("%-12s: \n", "Conflicts")
	fmt.Println()
	fmt.Printf("%-6s: %s\n", "MD5", c.MD5)
	fmt.Printf("%-6s: %s\n", "SHA1", c.SHA1)
	fmt.Printf("%-6s: %s\n", "SHA256", c.SHA256)

	return nil
}

func showRPM(file string) error {
	return fmt.Errorf("not yet implemented")
}
