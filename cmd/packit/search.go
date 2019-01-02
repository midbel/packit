package main

import (
  "fmt"
  "os"
  "path/filepath"
  "strings"
  "text/tabwriter"

  "github.com/midbel/cli"
  "github.com/midbel/packit/deb/control"
)

const (
  dpkgBase = "/var/lib/dpkg"
)
var dpkgStatus = filepath.Join(dpkgBase, "status")

func runSearch(cmd *cli.Command, args []string) error {
  kind := cmd.Flag.String("k", "", "package type")
  if err := cmd.Flag.Parse(args); err != nil {
    return err
  }
  switch *kind {
  case "", "rpm":
    return fmt.Errorf("%s not yet supported", *kind)
  case "deb":
    return searchDebPackage(cmd.Flag.Arg(0))
  }
  return nil
}

func searchDebPackage(n string) error {
  r, err := os.Open(dpkgStatus)
  if err != nil {
    return err
  }
  defer r.Close()

  cs, err := control.ParseMulti(r)
  if err != nil {
    return err
  }
	w := tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
	defer w.Flush()
  for _, c := range cs {
    if !strings.Contains(c.Package, n) {
      continue
    }
    fmt.Fprintf(w, "%s\t%s\t%s\n", c.Package, c.Version, c.Summary)
  }
  return nil
}
