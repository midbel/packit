package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/packit"
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
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.PackageType(), p.PackageName(), c.Version, c.Summary)
		return nil
	})
}

func showDescription(ns []string) error {
	const meta = `{{.Control.Package}}
{{with .Control}}
- type        : {{$.Type}}
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
	n := len(ns)
	var i int
	return showPackages(ns, func(p packit.Package) error {
		i++
		c := struct {
			Type    string
			Index   int
			Total   int
			Control packit.Control
		}{
			Type:    p.PackageType(),
			Index:   i,
			Total:   n,
			Control: p.About(),
		}
		return t.Execute(os.Stdout, c)
	})
}

func runVerify(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
	defer w.Flush()
	return showPackages(cmd.Flag.Args(), func(p packit.Package) error {
		var status string
		c := p.About()
		if err := p.Valid(); err != nil {
			status = err.Error()
		} else {
			status = "OK"
		}
		fmt.Fprintf(w, "%s\t%s (%s)\t%s\n", p.PackageType(), p.PackageName(), c.Version, status)
		return nil
	})
}
