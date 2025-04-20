package git

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	gitDir     = ".git"
	refsDir    = "refs"
	tagsDir    = "tags"
	headsDir   = "heads"
	remotesDir = "remotes"
	headFile   = "HEAD"
	origin     = "origin"
	master     = "main"
)

func LocalBranches() []string {
	dir := filepath.Join(gitDir, refsDir, headsDir)
	return readDir(dir)
}

func RemoteBranches() []string {
	dir := filepath.Join(gitDir, refsDir, remotesDir)
	return readDir(dir)
}

func CurrentBranch() string {
	file := filepath.Join(gitDir, headFile)
	buf, err := os.ReadFile(file)
	if err != nil {
		return master
	}

	_, path, ok := strings.Cut(string(buf), ":")
	if !ok {
		return master
	}
	return filepath.Base(strings.TrimSpace(path))
}

func Tags() []string {
	return getTags()
}

func CurrentTag() string {
	tags := getTags()
	if len(tags) == 0 {
		return ""
	}
	return tags[len(tags)-1]
}

func LookupConfig(path []string, scope string) string {
	return ""
}

func User() string {
	return ""
}

func Email() string {
	return ""
}

func Remote(name string) string {
	return ""
}

func Origin() string {
	return Remote(origin)
}

func getTags() []string {
	dir := filepath.Join(gitDir, refsDir, tagsDir)
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	modTime := func(e os.DirEntry) time.Time {
		i, err := e.Info()
		if err != nil {
			return time.Now()
		}
		return i.ModTime()
	}
	slices.SortFunc(es, func(a, b os.DirEntry) int {
		t1 := modTime(a)
		t2 := modTime(b)
		return int(t1.Unix() - t2.Unix())
	})
	var tags []string
	for _, e := range es {
		tags = append(tags, e.Name())
	}
	return tags
}

func readDir(dir string) []string {
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var list []string
	for _, e := range es {
		list = append(list, e.Name())
	}
	return list
}
