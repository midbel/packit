package main

import (
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/midbel/distance"
	"github.com/midbel/packit/internal/build"
	"github.com/midbel/packit/internal/packfile"
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
	"show-files": runFiles,
	"show-dependencies": runDependencies,
}

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "build, inspect and verify deb and/or rpm packages easily")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "available commands:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  build     create rpm/deb packages (alias: make)")
		fmt.Fprintln(os.Stderr, "  inspect   display package information (alias: info, show)")
		fmt.Fprintln(os.Stderr, "  verify    check integrity of a package (alias: check)")
		fmt.Fprintln(os.Stderr, "  content   list of files in a package")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "usage: packit <command> [<args>]")
		os.Exit(2)
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

func runDependencies(args []string) error {
	var (
		set = flag.NewFlagSet("show-dependencies", flag.ExitOnError)
		file = set.String("f", "Packfile", "package file")
	)
	set.Usage = func() {
		fmt.Fprintln(os.Stderr, "show dependencies required by the final package")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -f  path to the Packfile used to build the package - default to Packfile in the current working directory")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: packit show-files [OPTIONS] <CONTEXT>")
		fmt.Fprintln(os.Stderr)
		os.Exit(2)
	}
	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() == 0 {
		return fmt.Errorf("missing context")
	}
	pkg, err := packfile.Load(*file, set.Arg(0))
	if err != nil {
		return err
	}	
	_ = pkg
	return nil
}

func runFiles(args []string) error {
	var (
		set = flag.NewFlagSet("show-files", flag.ExitOnError)
		file = set.String("f", "Packfile", "package file")
	)
	set.Usage = func() {
		fmt.Fprintln(os.Stderr, "show files that will be included into the final package")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -f  path to the Packfile used to build the package - default to Packfile in the current working directory")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: packit show-files [OPTIONS] <CONTEXT>")
		fmt.Fprintln(os.Stderr)
		os.Exit(2)
	}
	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() == 0 {
		return fmt.Errorf("missing context")
	}
	pkg, err := packfile.Load(*file, set.Arg(0))
	if err != nil {
		return err
	}
	for _, r := range pkg.Files {
		fmt.Fprintln(os.Stdout, r.Path)
	}
	return nil
}

func runBuild(args []string) error {
	var (
		set  = flag.NewFlagSet("build", flag.ExitOnError)
		kind = set.String("k", "", "package type")
		file = set.String("f", "Packfile", "package file")
		dist = set.String("d", "", "directory where package will be written")
	)
	set.Usage = func() {
		fmt.Fprintln(os.Stderr, "build a new package")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Aliases:")
		fmt.Fprintln(os.Stderr, "  packit make")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -k  specify type of package to build - rpm or deb")
		fmt.Fprintln(os.Stderr, "  -f  path to the Packfile used to build the package - default to Packfile in the current working directory")
		fmt.Fprintln(os.Stderr, "  -d  folder where the final package will be saved")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: packit build [OPTIONS] <CONTEXT>")
		os.Exit(2)
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
	var (
		set       = flag.NewFlagSet("inspect", flag.ExitOnError)
		printAll  = set.Bool("a", false, "print all informations of package")
		printDeps = set.Bool("d", false, "print only dependencies of package")
	)
	set.Usage = func() {
		fmt.Fprintln(os.Stderr, "display information of the given package")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Aliases:")
		fmt.Fprintln(os.Stderr, "  packit show, packit info")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -d  print only the dependencies of the given package")
		fmt.Fprintln(os.Stderr, "  -a  print information and dependencies of the given package")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: packit inspect <PACKAGE>")
		os.Exit(2)
	}
	if err := set.Parse(args); err != nil {
		return err
	}
	return build.Info(set.Arg(0), *printAll, *printDeps, os.Stdout)
}

func runContent(args []string) error {
	set := flag.NewFlagSet("content", flag.ExitOnError)
	set.Usage = func() {
		fmt.Fprintln(os.Stderr, "display files and directories of the given package")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: packit content <PACKAGE>")
		os.Exit(2)
	}
	if err := set.Parse(args); err != nil {
		return err
	}
	return build.Content(set.Arg(0), os.Stdout)
}

func runVerify(args []string) error {
	set := flag.NewFlagSet("verify", flag.ExitOnError)
	set.Usage = func() {
		fmt.Fprintln(os.Stderr, "verify integrity of the given package")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Aliases:")
		fmt.Fprintln(os.Stderr, "  packit check")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: packit verify <PACKAGE>")
		os.Exit(2)
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
