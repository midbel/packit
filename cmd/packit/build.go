package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

func runConvert(cmd *cli.Command, args []string) error {
	who := cmd.Flag.String("m", "", "maintainer")
	datadir := cmd.Flag.String("d", os.TempDir(), "data directory")
	format := cmd.Flag.String("k", "", "package format")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	switch *format {
	case "", "deb", "rpm":
	default:
		return fmt.Errorf("unsupported packet type %s", *format)
	}
	var maintainer *packit.Maintainer
	switch *who {
	case "", "-":
	case "default":
		maintainer = &packit.DefaultMaintainer
	default:
		m, err := packit.ParseMaintainer(*who)
		if err != nil {
			return err
		}
		maintainer = m
	}
	return showPackages(cmd.Flag.Args(), func(p packit.Package) error {
		if p.PackageType() == *format {
			return nil
		}
		workdir := filepath.Join(os.TempDir(), p.PackageType(), p.PackageName())
		if err := os.RemoveAll(workdir); err != nil {
			return err
		}
		if err := os.MkdirAll(workdir, 0755); err != nil && !os.IsExist(err) {
			return err
		}
		if err := p.Extract(workdir, false); err != nil {
			return err
		}
		var mf packit.Makefile
		filepath.Walk(workdir, func(p string, i os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if i.IsDir() {
				return nil
			}
			f := packit.File{
				Src:  p,
				Dst:  strings.TrimPrefix(p, workdir),
				Name: filepath.Base(p),
				Perm: int(i.Mode()),
				Conf: packit.IsConfFile(p),
			}
			mf.Files = append(mf.Files, &f)
			return nil
		})
		c := p.About()
		if maintainer != nil {
			c.Maintainer = maintainer
		}
		mf.Control = &c
		for _, c := range p.History() {
			mf.Changes = append(mf.Changes, &c)
		}

		b, err := buildPackage(&mf, *format)
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

func runPack(cmd *cli.Command, args []string) error {
	datadir := cmd.Flag.String("d", os.TempDir(), "data directory")
	format := cmd.Flag.String("k", "", "packet type")
	merge := cmd.Flag.Bool("m", false, "merge given packages into an uniq package")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	if ns := cmd.Flag.Args(); *merge {
		return mergePackages(ns[1:], ns[0], *datadir, *format)
	} else {
		return repackPackages(ns, *datadir, *format)
	}
}

func mergePackages(pkgs []string, name, datadir, format string) error {
	return fmt.Errorf("merge not yet implemented")
}

func repackPackages(pkgs []string, datadir, format string) error {
	var group errgroup.Group
	for _, a := range pkgs {
		a := a
		group.Go(func() error {
			mf, err := makefile(a)
			if err != nil {
				return err
			}
			b, err := buildPackage(mf, format)
			if err != nil {
				return err
			}
			w, err := os.Create(filepath.Join(datadir, b.PackageName()))
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
	var ctrl *packit.Control
	for _, c := range cs {
		if c.Package == n {
			ctrl = c
			break
		}
	}
	if ctrl == nil {
		return nil, fmt.Errorf("%s not found", n)
	}
	sort.Strings(ctrl.Provides)
	if ix := sort.SearchStrings(ctrl.Provides, n); (ix < len(ctrl.Provides) && ctrl.Provides[ix] != n) || ix >= len(ctrl.Provides) {
		ctrl.Provides = append(ctrl.Provides, n)
		sort.Strings(ctrl.Provides)
	}
	return ctrl, nil
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
