package main

import (
	"log"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/packit"
	"github.com/midbel/toml"
	"golang.org/x/sync/errgroup"
)

var commands = []*cli.Command{
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
		Usage: "show <package>",
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
		Usage: "changelog [-m maintainer] [-c count] [-f from] [-t to] <package,...>",
		Alias: []string{"log"},
		Short: "dump changelog of given package",
		Run:   nil,
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
	return cmd.Flag.Parse(args)
}

func runShow(cmd *cli.Command, args []string) error {
	const meta = `{{.File}}
{{with .Control}}
- name        : {{.Package}}
- version     : {{.Version}}
- size        : {{.Size}}
- architecture: {{.Arch | arch}}
- build-date  : {{.Date | datetime}}
- vendor      : {{.Vendor}}
- section     : {{.Section}}
- home        : {{.Home}}
- license     : {{.License}}
- summary     : {{.Summary}}

{{.Desc}}{{end}}
{{if gt .Total 1 }}{{if lt .Index .Total}}---{{end}}
{{end}}`
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	fs := template.FuncMap{
		"arch":     packit.ArchString,
		"datetime": func(t time.Time) string { return t.Format("Mon, 02 Jan 2006 15:04:05 -0700") },
	}
	t, err := template.New("desc").Funcs(fs).Parse(meta)
	if err != nil {
		return err
	}
	for i, a := range cmd.Flag.Args() {
		p, err := packit.Open(a)
		if err != nil {
			return err
		}
		mf := p.Metadata()
		c := struct {
			Index   int
			Total   int
			File    string
			Control *packit.Control
		}{
			Index:   i,
			Total:   cmd.Flag.NArg(),
			File:    filepath.Base(a),
			Control: mf.Control,
		}
		if err := t.Execute(os.Stdout, c); err != nil {
			return err
		}
	}
	return nil
}

func runConvert(cmd *cli.Command, args []string) error {
	return nil
}

func runVerify(cmd *cli.Command, args []string) error {
	return nil
}

func runBuild(cmd *cli.Command, args []string) error {
	format := cmd.Flag.String("k", "", "package format")
	datadir := cmd.Flag.String("d", os.TempDir(), "datadir")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}

	if err := os.MkdirAll(*datadir, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	var group errgroup.Group
	for _, a := range cmd.Flag.Args() {
		a := a
		group.Go(func() error {
			r, err := os.Open(a)
			if err != nil {
				return err
			}
			defer r.Close()

			var mf packit.Makefile
			if err := toml.NewDecoder(r).Decode(&mf); err != nil {
				return err
			}
			b, err := packit.Prepare(&mf, *format)
			if err != nil {
				return err
			}
			w, err := os.Create(filepath.Join(*datadir, b.PackageName()))
			if err != nil {
				return err
			}
			defer w.Close()
			return b.Build(w)
		})
	}
	return group.Wait()
}
