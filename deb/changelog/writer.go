package changelog

import (
	"compress/gzip"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/packit"
)

const debChangelog = `{{range .changes}}  {{$.Package}} ({{.Version}}) {{.Distrib | join " "}}; urgency=low

{{range .changes}}   * {{.}}
{{end}}
  -- {{.Maintainer.Name}} <{{.Maintainer.Email}}> {{.When | datetime}}
{{end}}`

const debDateFormat = "Mon, 02 Jan 2006 15:04:05 -0700"

func DumpCompressed(name string, cs []*packit.Change, w io.Writer) error {
	ww, _ := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err := Dump(name, cs, ww); err != nil {
		return err
	}
	return ww.Close()
}

func Dump(name string, cs []*packit.Change, w io.Writer) error {
	fmap := template.FuncMap{
		"join": strings.Join,
		"datetime": func(t time.Time) string {
			if t.IsZero() {
				t = time.Now()
			}
			return t.Format(debDateFormat)
		},
	}
	t, err := template.New("changelog").Funcs(fmap).Parse(debChangelog)
	if err != nil {
		return err
	}
	c := struct {
		Package string
		Changes []*packit.Change
	}{
		Package: name,
		Changes: cs,
	}
	return t.Execute(w, c)
}
