package glob

import (
	"errors"
	"strings"
	"io/fs"
	"os"
)

func Walk(pattern, root string) ([]string, error) {
	if !hasSpecial(pattern) {
		return []string{pattern}, nil
	}
	pt, err := parseLine(pattern)
	if err != nil {
		return nil, err
	}
	var list []string
	fs.WalkDir(os.DirFS(root), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		err = pt.Match(path); 
		if err == nil {
			list = append(list, path)
		}
		if errors.Is(err, ErrIgnore) {
			err = nil
		}
		return err
	})
	return list, nil
}

func hasSpecial(str string) bool {
	return strings.IndexAny(str, "[*?") >= 0
}
