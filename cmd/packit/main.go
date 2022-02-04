package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/midbel/cli"
	"github.com/midbel/packit"
	"github.com/midbel/packit/deb"
	"github.com/midbel/packit/rpm"
)

var commands = []*cli.Command{
	{
		Usage:   "build [-k <type>] [-d <directory>] <config.fig>",
		Short:   "build package from its configuration file",
		Alias:   []string{"make"},
		Run:     runBuild,
		Default: true,
	},
	{
		Usage: "convert [-k <type>] [-d <directory>] <package>",
		Short: "convert a package from one format into another one",
		Alias: []string{"transform"},
		Run:   runConvert,
	},
	{
		Usage: "extract [-d <directory>] [-f <flat>] [-a <all>] package",
		Short: "extract files from package archive",
		Run:   runExtract,
	},
	{
		Usage: "info <package>",
		Short: "show information on a package",
		Alias: []string{"show"},
		Run:   runInfo,
	},
	{
		Usage: "list <package>",
		Short: "list content of a package",
		Alias: []string{"content"},
		Run:   runList,
	},
	{
		Usage: "verify <package>",
		Short: "check integrity of a package",
		Alias: []string{"check"},
		Run:   runVerify,
	},
}

const helpText = `{{.Name}} help to create packages in various format such as  deb
or rpm (and maybe other in a later time)

Usage: {{.Name}} command [arguments]

Available commands:

{{range .Commands}}{{printf "  %-9s %s" .String .Short}}
{{end}}
Use {{.Name}} [command] -h for more information about its usage.
`

func main() {
	cli.RunAndExit(commands, cli.Usage("packit", helpText, commands))
}

func runBuild(cmd *cli.Command, args []string) error {
	var (
		dir  = cmd.Flag.String("d", "", "output directory")
		kind = cmd.Flag.String("k", "", "package type")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		fmt.Println("oups oups", err, args)
		return err
	}
	r, err := os.Open(cmd.Flag.Arg(0))
	if err != nil {
		return err
	}
	m, err := packit.Load(r, *kind)
	if err != nil {
		return err
	}
	switch *kind {
	case packit.DEB, "":
		err = deb.Build(*dir, m)
	case packit.RPM:
		err = rpm.Build(*dir, m)
	default:
		err = fmt.Errorf("%s: %w", *kind, packit.ErrPackage)
	}
	return err
}

func runConvert(cmd *cli.Command, args []string) error {
	var (
		dir  = cmd.Flag.String("d", "", "directory")
		kind = cmd.Flag.String("k", "", "kind")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	_, _ = dir, kind
	return nil
}

func runExtract(cmd *cli.Command, args []string) error {
	var (
		dir  = cmd.Flag.String("d", "", "directory")
		all  = cmd.Flag.Bool("a", false, "extract all")
		flat = cmd.Flag.Bool("f", false, "flat")
	)
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var err error
	switch ext := Ext(cmd.Flag.Arg(0)); ext {
	case packit.RPM:
		err = rpm.Extract(cmd.Flag.Arg(0), *dir, *flat, *all)
	case packit.DEB:
		err = deb.Extract(cmd.Flag.Arg(0), *dir, *flat, *all)
	default:
		err = fmt.Errorf("%s: %w", cmd.Flag.Arg(0), packit.ErrPackage)
	}
	return err
}

func runList(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var (
		err  error
		list []packit.Resource
	)
	switch ext := Ext(cmd.Flag.Arg(0)); ext {
	case packit.RPM:
		list, err = rpm.List(cmd.Flag.Arg(0))
	case packit.DEB:
		list, err = deb.List(cmd.Flag.Arg(0))
	default:
		err = fmt.Errorf("%s: %w", cmd.Flag.Arg(0), packit.ErrPackage)
	}
	if err == nil {
		printResources(list)
	}
	return err
}

func runInfo(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var (
		err  error
		meta packit.Metadata
	)
	switch ext := Ext(cmd.Flag.Arg(0)); ext {
	case packit.RPM:
		meta, err = rpm.Info(cmd.Flag.Arg(0))
	case packit.DEB:
		meta, err = deb.Info(cmd.Flag.Arg(0))
	default:
		err = fmt.Errorf("%s: %w", cmd.Flag.Arg(0), packit.ErrPackage)
	}
	if err == nil {
		printMetadata(meta)
	}
	return err
}

func runVerify(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var err error
	switch ext := Ext(cmd.Flag.Arg(0)); ext {
	case packit.RPM:
		err = rpm.Verify(cmd.Flag.Arg(0))
	case packit.DEB:
		err = deb.Verify(cmd.Flag.Arg(0))
	default:
		err = fmt.Errorf("%s: %w", cmd.Flag.Arg(0), packit.ErrPackage)
	}
	return err
}

func printMetadata(meta packit.Metadata) {

}

func printResources(list []packit.Resource) {

}

func Ext(file string) string {
	ext := filepath.Ext(file)
	return strings.TrimPrefix(ext, ".")
}
