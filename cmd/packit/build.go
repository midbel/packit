package main

import (
  "fmt"
  "os"
  "path/filepath"

  "golang.org/x/sync/errgroup"
  "github.com/midbel/cli"
  "github.com/midbel/toml"
  "github.com/midbel/packit"
  "github.com/midbel/packit/deb"
  "github.com/midbel/packit/rpm"
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
  switch format {
  case "deb", "":
    return deb.Build(&mf)
  case "rpm":
    return rpm.Build(&mf)
  default:
    return nil, fmt.Errorf("unsupported package type")
  }
}
