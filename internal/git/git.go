package git

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	gitDir        = ".git"
	refsDir       = "refs"
	tagsDir       = "tags"
	headsDir      = "heads"
	remotesDir    = "remotes"
	headFile      = "HEAD"
	origin        = "origin"
	master        = "main"
	configFile    = "config"
	gitconfigFile = ".gitconfig"
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

func User() string {
	if gitConfig == nil {
		return ""
	}
	return gitConfig.GetUser()
}

func Email() string {
	if gitConfig == nil {
		return ""
	}
	return gitConfig.GetMail()
}

func Remote(name string) string {
	if gitConfig == nil {
		return ""
	}
	return gitConfig.GetRemoteURL(name)
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

type Section struct {
	Name   string
	Sub    string
	Values map[string]string
}

type Config struct {
	Sections []Section
}

func (c *Config) GetUser() string {
	ix := slices.IndexFunc(c.Sections, func(s Section) bool {
		return s.Name == "user"
	})
	if ix < 0 {
		return ""
	}
	return c.Sections[ix].Values["name"]
}

func (c *Config) GetMail() string {
	ix := slices.IndexFunc(c.Sections, func(s Section) bool {
		return s.Name == "user"
	})
	if ix < 0 {
		return ""
	}
	return c.Sections[ix].Values["email"]
}

func (c *Config) GetRemoteURL(name string) string {
	ix := slices.IndexFunc(c.Sections, func(s Section) bool {
		return s.Name == "remote" && s.Sub == name
	})
	if ix < 0 {
		return ""
	}
	return c.Sections[ix].Values["url"]
}

var gitConfig *Config

func Load() error {
	cfg, err := readConfig()
	if err == nil {
		gitConfig = cfg
	}
	return err
}

func readConfig() (*Config, error) {
	var (
		cfg    Config
		dir, _ = os.UserHomeDir()
	)
	if err := readConfigFromFile(filepath.Join(dir, gitconfigFile), &cfg); err != nil {
		return nil, err
	}
	return &cfg, readConfigFromFile(filepath.Join(gitDir, configFile), &cfg)
}

func readConfigFromFile(file string, config *Config) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	var (
		scan  = bufio.NewScanner(r)
		where int
	)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			var (
				name = line[1 : len(line)-1]
				sect Section
			)
			if name, sub, ok := strings.Cut(name, " "); ok {
				sect.Name, sect.Sub = name, sub
			} else {
				sect.Name = name
			}
			sect.Values = make(map[string]string)
			where = slices.IndexFunc(config.Sections, func(s Section) bool {
				return sect.Name == s.Name && sect.Sub == s.Sub
			})
			if where < 0 {
				config.Sections = append(config.Sections, sect)
				where = len(config.Sections) - 1
			}
			continue
		}
		option, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("line: missing =")
		}
		value, _, _ = strings.Cut(value, "#")

		option = strings.TrimSpace(option)
		value = strings.TrimSpace(value)

		config.Sections[where].Values[option] = value
	}
	return nil
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
