package changelog

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/packit"
	"github.com/midbel/packit/rw"
)

// const debChangelog = `{{range .Changes}}  {{$.Package}} ({{.Version}}) {{.Distrib}}; urgency=low
//
// {{range .Changes}}   * {{.}}
// {{end}}
//   -- {{.Maintainer.Name}} <{{.Maintainer.Email}}> {{.When | datetime}}
// {{end}}`
const debChangelog = `{{range .Changes}}{{$.Package}} ({{.Version}}) {{.Distrib | join }}; urgency=low

{{if .Body}}{{.Body | indent}}{{end}}
{{range .Changes}}{{if .Body}}  [{{.Maintainer.Name | title}}]
{{.Body | indent}}{{end}}
{{end}} -- {{.Maintainer.Name | title}} <{{.Maintainer.Email}}>  {{.When | datetime}}

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
		"title":  strings.Title,
		"join":   joinDistrib,
		"indent": indentBody,
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

func indentBody(text string) string {
	s := bufio.NewScanner(strings.NewReader(text))
	s.Split(splitText)

	var body bytes.Buffer
	for s.Scan() {
		t := rw.WrapDefault(s.Text())
		io.WriteString(&body, indentPart(t))
	}
	return body.String()
}

func splitText(bs []byte, ateof bool) (int, []byte, error) {
	if ateof {
		return len(bs), bs, bufio.ErrFinalToken
	}
	ix := bytes.Index(bs, []byte{0x0a, 0x0a})
	if ix < 0 {
		return 0, nil, nil
	}
	vs := make([]byte, ix)
	copy(vs, bs)
	return ix + 2, vs, nil
}

func indentPart(text string) string {
	const (
		star  = "  * "
		space = "    "
	)

	np := true
	prefix := star

	text = strings.TrimSpace(text)
	s := bufio.NewScanner(strings.NewReader(text))
	var body bytes.Buffer
	for s.Scan() {
		t := strings.TrimSpace(s.Text())
		if len(t) == 0 {
			fmt.Fprintln(&body, space)
			np, prefix = true, star
			continue
		}
		fmt.Fprintln(&body, prefix+t)
		if np {
			np, prefix = false, space
		}
	}
	return body.String()
}

func joinDistrib(ds []string) string {
	if len(ds) == 0 {
		return packit.DefaultDistrib
	}
	return strings.Join(ds, " ")
}
