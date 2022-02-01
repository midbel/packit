package main

import (
	"fmt"
	"os"

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
		Usage: "extract [-d <directory>] [-a <extract-all>] package",
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

func main() {
	cli.RunAndExit(commands, func() {})
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
		err = fmt.Errorf("%s: unsupported package type", *kind)
	}
	return err
}

func runConvert(cmd *cli.Command, args []string) error {
	return nil
}

func runExtract(cmd *cli.Command, args []string) error {
	return nil
}

func runList(cmd *cli.Command, args []string) error {
	return nil
}

func runInfo(cmd *cli.Command, args []string) error {
	return nil
}

func runVerify(cmd *cli.Command, args []string) error {
	return nil
}
