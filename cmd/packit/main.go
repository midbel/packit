package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/packit/internal/build"
)

var commands = map[string]func([]string) error{
	"build":   runBuild,
	"make":    runBuild,
	"inspect": runInspect,
	"show":    runInspect,
	"check":   runVerify,
	"verify":  runVerify,
}

func main() {
	flag.Parse()
	cmd, ok := commands[flag.Arg(0)]
	if !ok {
		fmt.Fprintf(os.Stderr, "command %s not supported", flag.Arg(0))
		fmt.Fprintln(os.Stderr)
		os.Exit(3)
	}
	args := flag.Args()
	if err := cmd(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runBuild(args []string) error {
	var (
		set  = flag.NewFlagSet("build", flag.ExitOnError)
		kind = set.String("k", "", "package type")
		file = set.String("f", "Packfile", "package file")
		dist = set.String("d", "", "directory where package will be written")
	)
	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() == 0 {
		return fmt.Errorf("missing context")
	}

	return build.BuildPackage(*file, *dist, *kind, set.Arg(0))
}

func runInspect(args []string) error {
	set := flag.NewFlagSet("inspect", flag.ExitOnError)
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}

func runVerify(args []string) error {
	set := flag.NewFlagSet("verify", flag.ExitOnError)
	if err := set.Parse(args); err != nil {
		return err
	}
	err := build.CheckPackage(set.Arg(0))
	if err == nil {
		fmt.Fprintf(os.Stdout, "%s: package is valid", set.Arg(0))
		fmt.Fprintln(os.Stdout)
	}
	return err
}
