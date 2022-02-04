package apk

import (
  "io"
  "os"
  "path/filepath"

  "github.com/midbel/packit"
)

func Build(dir string, meta packit.Metadata) error {
	w, err := os.Create(filepath.Join(dir, getPackageName(meta)))
	if err != nil {
		return err
	}
	defer w.Close()
	return build(w, meta)
}

func build(w io.Writer, meta packit.Metadata) error {
  return nil
}

func getPackageName(meta packit.Metadata) string {
  return ""
}
