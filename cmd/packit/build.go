package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/midbel/cli"
	"github.com/midbel/packit"
	"github.com/midbel/packit/deb"
	"github.com/midbel/packit/deb/control"
	"github.com/midbel/packit/rpm"
	"github.com/midbel/toml"
	"golang.org/x/sync/errgroup"
)

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
		if s, err := os.Stat(a); err != nil {
			continue
		} else {
			if !s.Mode().IsRegular() {
				return fmt.Errorf("%s: not a makefile", a)
			}
		}
		a := a
		group.Go(func() error {
			b, err := prepare(a, *format)
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

func runPack(cmd *cli.Command, args []string) error {
	datadir := cmd.Flag.String("d", os.TempDir(), "data directory")
	format := cmd.Flag.String("k", "", "packet type")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var group errgroup.Group
	for _, a := range cmd.Flag.Args() {
		a := a
		group.Go(func() error {
			mf, err := makefile(a)
			if err != nil {
				return err
			}
			b, err := buildPackage(mf, *format)
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

func prepare(file, format string) (packit.Builder, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var mf packit.Makefile
	if err := toml.NewDecoder(r).Decode(&mf); err != nil {
		return nil, err
	}
	return buildPackage(&mf, format)
}

func buildPackage(mf *packit.Makefile, format string) (packit.Builder, error) {
	switch format {
	case "deb", "":
		return deb.Build(mf)
	case "rpm":
		return rpm.Build(mf)
	default:
		return nil, fmt.Errorf("unsupported package type")
	}
}

func makefile(n string) (*packit.Makefile, error) {
	c, err := findPackage(n)
	if err != nil {
		return nil, err
	}
	fs, err := listFiles(n)
	if err != nil {
		return nil, err
	}
	mf := &packit.Makefile{
		Control: c,
		Files:   fs,
	}
	return mf, nil
}

func findPackage(n string) (*packit.Control, error) {
	r, err := os.Open(filepath.Join(dpkgBase, "status"))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	cs, err := control.ParseMulti(r)
	if err != nil {
		return nil, err
	}
	for _, c := range cs {
		if c.Package == n {
			return c, nil
		}
	}
	return nil, fmt.Errorf("%s not found", n)
}

func listFiles(n string) ([]*packit.File, error) {
	r, err := os.Open(filepath.Join(dpkgInfo, n+".list"))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var fs []*packit.File
	s := bufio.NewScanner(r)
	for s.Scan() {
		switch f, err := packit.LocalFile(s.Text()); err {
		case nil:
			fs = append(fs, f)
		case packit.ErrSkip:
		default:
			return nil, err
		}
	}
	return fs, s.Err()
}
