package main

import (
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/midbel/distance"
	"github.com/midbel/packit/internal/build"
)

var commands = map[string]func([]string) error{
	"build":   runBuild,
	"make":    runBuild,
	"inspect": runInspect,
	"show":    runInspect,
	"info":    runInspect,
	"check":   runVerify,
	"verify":  runVerify,
	"content": runContent,
}

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stdout, "build, inspect and verify deb and/rpm packages easily")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "available commands:")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "  build     create rpm/deb packages (alias: make)")
		fmt.Fprintln(os.Stdout, "  inspect   display package information (alias: info, show)")
		fmt.Fprintln(os.Stdout, "  verify    check integrity of a package (alias: check)")
		fmt.Fprintln(os.Stdout, "  content   list of files in a package")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "usage: packit <command> [<args>]")
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		return
	}
	cmd, ok := commands[flag.Arg(0)]
	if !ok {
		fmt.Fprintf(os.Stderr, "packit %s is not a packit command!", flag.Arg(0))
		fmt.Fprintln(os.Stderr)

		it := maps.Keys(commands)
		others := distance.Levenshtein(flag.Arg(0), slices.Collect(it))
		if len(others) > 0 {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "The most similar commands are:")
		}
		for i := range others {
			fmt.Fprintf(os.Stderr, "- %s", others[i])
			fmt.Fprintln(os.Stderr)
		}

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
	set.Usage = func() {

	}
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
	set.Usage = func() {

	}
	if err := set.Parse(args); err != nil {
		return err
	}
	return build.Info(set.Arg(0), os.Stdout)
}

func runContent(args []string) error {
	set := flag.NewFlagSet("content", flag.ExitOnError)
	set.Usage = func() {

	}
	if err := set.Parse(args); err != nil {
		return err
	}
	return build.Content(set.Arg(0), os.Stdout)
}

func runVerify(args []string) error {
	set := flag.NewFlagSet("verify", flag.ExitOnError)
	set.Usage = func() {

	}
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
