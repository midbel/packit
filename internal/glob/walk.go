package glob

import (
	"errors"
	"io/fs"
	"iter"
	"os"
	"slices"
	"strings"
)

func Walk(pattern, root string) (iter.Seq[string], error) {
	if !hasSpecial(pattern) {
		return slices.Values([]string{pattern}), nil
	}
	pt, err := parseLine(pattern)
	if err != nil {
		return nil, err
	}
	it := func(yield func(string) bool) {
		fs.WalkDir(os.DirFS(root), ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			err = pt.Match(path)
			if errors.Is(err, ErrIgnore) {
				return nil
			}
			if !yield(path) {
				return fs.SkipAll
			}
			return err
		})
	}
	return it, nil
}

func hasSpecial(str string) bool {
	return strings.IndexAny(str, "[*?") >= 0
}
