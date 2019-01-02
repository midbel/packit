package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/midbel/cli"
	"github.com/midbel/packit"
	"github.com/midbel/packit/deb"
	"github.com/midbel/packit/rpm"
)

var commands = []*cli.Command{
	{
		Usage: "search <package>",
		Short: "search for a given package in a database (dpkg, rppmdb, packit)",
		Run: runSearch,
	},
	{
		Usage: "build [-d datadir] [-k pkg-type] <config.toml,...>",
		Alias: []string{"make"},
		Short: "build package(s) from configuration file",
		Run:   runBuild,
	},
	{
		Usage: "convert <package> <package>",
		Short: "convert a source package into a destination package format",
		Run:   runConvert,
	},
	{
		Usage: "show [-l] <package>",
		Alias: []string{"info"},
		Short: "show package metadata",
		Run:   runShow,
	},
	{
		Usage: "verify <package,...>",
		Alias: []string{"check"},
		Short: "check the integrity of the given package(s)",
		Run:   runVerify,
	},
	{
		Usage: "history [-v version] [-c count] [-f from] [-t to] <package,...>",
		Alias: []string{"log", "changelog"},
		Short: "dump changelog of given package",
		Run:   runLog,
	},
	{
		Usage: "extract [-d datadir] [-p] <package...>",
		Short: "extract files from package payload in given directory",
		Run:   runExtract,
	},
	{
		Usage: "install <package,...>",
		Short: "install package on the system",
		Run:   nil,
	},
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)
}

const helpText = `{{.Name}} is an easy to use package manager which can be used
to create softwares package in various format, show their content and/or verify
their integrity.

Usage:

  {{.Name}} command [arguments]

The commands are:

{{range .Commands}}{{printf "  %-9s %s" .String .Short}}
{{end}}

Use {{.Name}} [command] -h for more information about its usage.
`

func main() {
	log.SetFlags(0)
	usage := func() {
		data := struct {
			Name     string
			Commands []*cli.Command
		}{
			Name:     filepath.Base(os.Args[0]),
			Commands: commands,
		}
		t := template.Must(template.New("help").Parse(helpText))
		t.Execute(os.Stderr, data)

		os.Exit(2)
	}
	if err := cli.Run(commands, usage, nil); err != nil {
		log.Fatalln(err)
	}
}

func runLog(cmd *cli.Command, args []string) error {
	return nil
}

func runExtract(cmd *cli.Command, args []string) error {
	datadir := cmd.Flag.String("d", os.TempDir(), "datadir")
	preserve := cmd.Flag.Bool("p", false, "preserve")
	cleandir := cmd.Flag.Bool("r", false, "clean")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	return showPackages(cmd.Flag.Args(), func(p packit.Package) error {
		dir := filepath.Join(*datadir, p.PackageName())
		if *cleandir {
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
		}
		if err := p.Extract(dir, *preserve); err != nil {
			if *cleandir {
				os.RemoveAll(dir)
			}
			return err
		}
		return nil
	})
}

func runConvert(cmd *cli.Command, args []string) error {
	return cmd.Flag.Parse(args)
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
