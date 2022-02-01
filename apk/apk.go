package apk

import (
  "path/filepath"
  "os"

  "github.com/midbel/packit"
)

func Build(dir string, meta packit.Metadata) error {
  w, err := os.Create(filepath.Join(dir, getPackageName(meta)))
  if err != nil {
    return err
  }
  defer w.Close()
  return nil
}

func getPackageName(meta packit.Metadata) string {
  return ""
}
