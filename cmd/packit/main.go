package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/packit"
	"github.com/midbel/packit/deb"
	"github.com/midbel/packit/rpm"
)

var commands = []*cli.Command{
	{
		Usage: "build [-d datadir] [-k pkg-type] <config.toml,...>",
		Alias: []string{"make"},
		Short: "build package(s) from configuration file",
		Run:   runBuild,
	},
	{
		Usage: "convert [-m maintainer] [-d datadir] [-k type] <package>",
		Short: "convert a package into another package format",
		Run:   runConvert,
	},
	{
		Usage: "show [-l] <package>",
		Alias: []string{"info"},
		Short: "show package metadata",
		Run:   runShow,
	},
	{
		Usage: "verify <package...>",
		Alias: []string{"check"},
		Short: "check the integrity of the given package(s)",
		Run:   runVerify,
	},
	{
		Usage: "history [-w who] [-f from] [-t to] <package,...>",
		Alias: []string{"log", "changelog"},
		Short: "dump changelog of given package",
		Run:   runLog,
	},
	{
		Usage: "extract [-r remove] [-d datadir] [-p] <package...>",
		Short: "extract files from package payload in given directory",
		Run:   runExtract,
	},
	{
		Usage: "repack [-m] [-d datadir] [-k type] <package>",
		Short: "create a package from files installed on local system",
		Run:   runPack,
	},
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
	cli.RunAndExit(commands, cli.Usage("packit", helpText, commands))
}

func runLog(cmd *cli.Command, args []string) error {
	const history = `
Package     : {{.Package -}}
{{with .Change}}
Date        : {{.When | datetime}}
Version     : {{.Version}}
Distribution: {{.Distrib | join}}
Maintainer  : {{if .Maintainer}}{{.Maintainer.Name}}{{else}}unknown{{end}}
Changes     :
{{.Body -}}
{{end}}
`
	start := cmd.Flag.String("f", "", "")
	end := cmd.Flag.String("t", "", "")
	who := cmd.Flag.String("w", "", "")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var (
		fd, td time.Time
		err    error
	)
	if fd, err = time.Parse("2006-01-02", *start); err != nil && *start != "" {
		return err
	}
	if td, err = time.Parse("2006-01-02", *end); err != nil && *end != "" {
		return err
	}
	fs := template.FuncMap{
		"datetime": func(t time.Time) string {
			if t.IsZero() {
				t = time.Now()
			}
			return t.Format("2006-01-02 15:04:05")
		},
		"join": func(vs []string) string {
			if len(vs) == 0 {
				return "-"
			}
			return strings.Join(vs, ", ")
		},
	}
	t, err := template.New("changelog").Funcs(fs).Parse(strings.TrimSpace(history))
	if err != nil {
		return err
	}
	return showPackages(cmd.Flag.Args(), func(p packit.Package) error {
		cs := p.History().Filter(*who, fd, td)
		for i, c := range cs {
			v := struct {
				Package string
				Change  packit.Change
			}{
				Package: p.PackageName(),
				Change:  c,
			}
			if err := t.Execute(os.Stdout, v); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout)
			if i < len(cs)-1 {
				fmt.Fprintln(os.Stdout, "--")
			}
		}
		if len(cs) > 0 {
			fmt.Fprintln(os.Stdout)
		}
		return nil
	})
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

func showControls(ns []string, fn func(packit.Control) error) error {
	if fn == nil {
		return nil
	}
	for _, n := range ns {
		var (
			c   *packit.Control
			err error
		)
		switch e := filepath.Ext(n); e {
		case ".deb":
			c, err = deb.About(n)
		case ".rpm":
			c, err = rpm.About(n)
		default:
		}
		if err != nil {
			return err
		}
		if err := fn(*c); err != nil {
			return fmt.Errorf("%s: %s", c.Package, err)
		}
	}
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
			return fmt.Errorf("fail to read %s: %s", n, err)
		}
		if err := fn(pkg); err != nil {
			return fmt.Errorf("%s: %s", pkg.PackageName(), err)
		}
	}
	return nil
}
