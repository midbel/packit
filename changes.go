package packit

import (
	"sort"
	"strings"
	"time"
)

type Change struct {
	When        time.Time `toml:"date"`
	Body        string    `toml:"description"`
	Version     string    `toml:'version'`
	Distrib     []string  `toml:"distrib"`
	Changes     []Change  `toml:"changes"`
	*Maintainer `toml:"maintainer"`
}

type History []Change

func (h History) All() []Change {
	var t time.Time
	return h.Filter("", t, t)
}

func (h History) Between(fd, td time.Time) []Change {
	return h.Filter("", fd, td)
}

func (h History) Filter(who string, fd, td time.Time) []Change {
	var cs []Change
	for _, c := range h {
		m := who == "" || (c.Maintainer != nil && strings.Contains(c.Maintainer.Name, who))
		f := fd.IsZero() || (c.When.After(fd) || c.When.Equal(fd))
		t := td.IsZero() || (c.When.Before(td) || c.When.Equal(td))

		if f && t && m {
			cs = append(cs, c)
		}
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].When.After(cs[j].When) })
	return cs
}
