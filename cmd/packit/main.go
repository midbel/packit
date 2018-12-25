package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
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
		Usage: "history [-c count] [-f from] [-t to] <package,...>",
		Alias: []string{"log", "changelog"},
		Short: "dump changelog of given package",
		Run:   runLog,
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

func showMakefile(pkgs []string, show func(i int, mf *packit.Makefile) error) error {
	for i, a := range pkgs {
		p, err := packit.Open(a)
		if err != nil {
			return err
		}
		mf := p.Metadata()
		if err := show(i, mf); err != nil {
			return err
		}
	}
	return nil
}

func runLog(cmd *cli.Command, args []string) error {
	const meta = `{{.File}}
{{range .Changes}}
{{ .When | datetime }} ({{ if .Maintainer}}{{ .Maintainer.Name }}{{else}}unknown{{end}})
  {{ .Changes | join }}
{{end}}{{if gt .Total 1 }}{{if lt .Index .Total}}---{{end}}
{{end}}`
	dtstart := cmd.Flag.String("f", "", "from date")
	dtend := cmd.Flag.String("t", "", "to date")
	count := cmd.Flag.Int("c", 0, "count")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var (
		fd, td time.Time
		err    error
	)
	if fd, err = time.Parse("2006-01-02", *dtstart); err != nil && *dtstart != "" {
		return err
	}
	if td, err = time.Parse("2006-01-02", *dtend); err != nil && *dtend != "" {
		return err
	}
	fs := template.FuncMap{
		"join":     func(s []string) string { return strings.Join(s, "\n  ") },
		"datetime": func(t time.Time) string { return t.Format("2006-01-02") },
	}
	t, err := template.New("desc").Funcs(fs).Parse(meta)
	if err != nil {
		return err
	}
	n := cmd.Flag.NArg()
	return showMakefile(cmd.Flag.Args(), func(i int, mf *packit.Makefile) error {
		if len(mf.Changes) == 0 {
			return nil
		}
		var cs []*packit.Change
		switch {
		case fd.IsZero() && td.IsZero():
			cs = mf.Changes
		case fd.IsZero() && !td.IsZero():
			for _, c := range mf.Changes {
				if c.When.After(td) {
					continue
				}
				cs = append(cs, c)
			}
		case td.IsZero() && !fd.IsZero():
			for _, c := range mf.Changes {
				if c.When.Before(fd) {
					continue
				}
				cs = append(cs, c)
			}
		default:
			for _, c := range mf.Changes {
				if c.When.Before(fd) || c.When.After(td) {
					continue
				}
				cs = append(cs, c)
			}
		}
		if *count > 0 && len(cs) >= *count {
			cs = cs[:*count]
		}
		c := struct {
			Index   int
			Total   int
			File    string
			Changes []*packit.Change
		}{
			Index:   i + 1,
			Total:   n,
			File:    mf.PackageName(),
			Changes: cs,
		}
		return t.Execute(os.Stdout, c)
	})
}

func runShow(cmd *cli.Command, args []string) error {
	long := cmd.Flag.Bool("l", false, "show full package description")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	if args := cmd.Flag.Args(); *long {
		return showMetadata(args)
	} else {
		return showSummary(args)
	}
}

func showMetadata(pkgs []string) error {
	const meta = `{{.File}}
{{with .Control}}
- name        : {{.Package}}
- version     : {{.Version}}
- size        : {{.Size}}
- maintainer  : {{.Maintainer}}
- architecture: {{.Arch | arch}}
- build-date  : {{.Date | datetime}}
- vendor      : {{if .Vendor}}{{.Vendor}}{{else}}-{{end}}
- section     : {{.Section}}
- home        : {{if.Home}}{{.Home}}{{else}}-{{end}}
- license     : {{if .License}}{{.License}}{{else}}-{{end}}
- summary     : {{.Summary}}

{{.Desc}}{{end}}
{{if gt .Total 1 }}{{if lt .Index .Total}}---{{end}}
{{end}}`
	fs := template.FuncMap{
		"arch":     packit.ArchString,
		"datetime": func(t time.Time) string { return t.Format("Mon, 02 Jan 2006 15:04:05 -0700") },
	}
	t, err := template.New("desc").Funcs(fs).Parse(meta)
	if err != nil {
		return err
	}
	n := len(pkgs)
	return showMakefile(pkgs, func(i int, mf *packit.Makefile) error {
		c := struct {
			Index   int
			Total   int
			File    string
			Control *packit.Control
		}{
			Index:   i + 1,
			Total:   n,
			File:    mf.PackageName(),
			Control: mf.Control,
		}
		return t.Execute(os.Stdout, c)
	})
}

func showSummary(pkgs []string) error {
	w := tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
	defer w.Flush()

	return showMakefile(pkgs, func(_ int, mf *packit.Makefile) error {
		fmt.Fprintf(w, "%s\t%s\n", mf.PackageName(), mf.Control.Summary)
		return nil
	})
}

func runConvert(cmd *cli.Command, args []string) error {
	return cmd.Flag.Parse(args)
}

func runVerify(cmd *cli.Command, args []string) error {
	return cmd.Flag.Parse(args)
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
