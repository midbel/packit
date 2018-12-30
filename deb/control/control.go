package control

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/midbel/packit"
	"github.com/midbel/packit/rw"
)

var (
	ErrSyntax  = errors.New("invalid syntax")
	ErrUnknown = errors.New("unknown field")
)

const debDateFormat = "Mon, 02 Jan 2006 15:04:05 -0700"

const debControl = `
Package: {{.Package}}
Version: {{.Version}}
License: {{.License}}
Section: {{.Section}}
Priority: {{.Priority}}
Date: {{.Date | datetime}}
Architecture: {{arch .Arch}}
Vendor: {{.Vendor}}
Maintainer: {{.Name}} <{{.Email}}>
Homepage: {{.Home}}
{{if .Depends }}Depends: {{join .Depends ", "}}{{end}}
{{if .Suggests }}Suggests: {{join .Suggests ", "}}{{end}}
Installed-Size: {{.Size | bytesize}}
Build-Using: {{.Compiler}}
Description: {{if .Summary }}{{.Summary}}{{else}}summary missing{{end}}
{{if .Desc }}{{indent .Desc}}{{end}}
`

func Dump(c *packit.Control, w io.Writer) error {
	fmap := template.FuncMap{
		"join":     strings.Join,
		"arch":     arch,
		"indent":   indent,
		"datetime": datetime,
		"bytesize": bytesize,
	}
	t, err := template.New("control").Funcs(fmap).Parse(strings.TrimSpace(debControl) + "\n")
	if err != nil {
		return err
	}
	return t.Execute(rw.Clean(w), c)
}

func Parse(r io.Reader) (*packit.Control, error) {
	var rs io.RuneScanner
	if x, ok := r.(io.RuneScanner); ok {
		rs = x
	} else {
		rs = bufio.NewReader(r)
	}
	var c packit.Control
	return &c, parseControl(rs, func(k, v string) error {
		switch strings.ToLower(k) {
		default:
			return ErrUnknown
		case "package":
			c.Package = v
		case "version":
			c.Version = v
		case "license":
			c.License = v
		case "section":
			c.Section = v
		case "priority":
			c.Priority = v
		case "architecture":
			switch v {
			case "amd64":
				c.Arch = packit.Arch64
			case "i386":
				c.Arch = packit.Arch32
			case "all":
				c.Arch = packit.ArchAll
			}
		case "date":
			d, err := time.Parse(debDateFormat, v)
			if err != nil {
				return err
			}
			c.Date = d
		case "vendor":
			c.Vendor = v
		case "maintainer":
			// c.Maintainer = parseMaintainer(v)
		case "homepage":
			c.Home = v
		case "depends":
			c.Depends = strings.Split(v, ", ")
		case "installed-size":
			s, err := strconv.ParseInt(v, 0, 64)
			if err != nil {
				return err
			}
			c.Size = s << 10
		case "build-using":
			c.Compiler = v
		case "description":
			ps := strings.SplitN(v, "\n", 2)
			switch len(ps) {
			case 1:
				c.Summary = ps[0]
			case 2:
				c.Summary, c.Desc = ps[0], ps[1]
			}
		}
		return nil
	})
}

const (
	nl     = '\n'
	space  = ' '
	hyphen = '-'
	colon  = ':'
	dot    = '.'
	tab    = '\t'
	hash   = '#'
)

func parseControl(rs io.RuneScanner, fn func(k, v string) error) error {
	for {
		if r, _, _ := rs.ReadRune(); r != nl {
			rs.UnreadRune()
		}
		k, err := parseKey(rs)
		if err != nil {
			return err
		}
		v, err := parseValue(rs)
		if err != nil {
			return err
		}
		if k == "" || v == "" {
			break
		}
		if err := fn(strings.TrimSpace(k), strings.TrimSpace(v)); err != nil {
			return err
		}
	}
	return nil
}

func parseKey(rs io.RuneScanner) (string, error) {
	if r, _, _ := rs.ReadRune(); r == hash {
		for r, _, _ := rs.ReadRune(); r != nl; r, _, _ = rs.ReadRune() {
		}
		return parseKey(rs)
	} else if r == hyphen {
		return "", ErrSyntax
	} else {
		rs.UnreadRune()
	}
	var k bytes.Buffer
	for {
		r, _, err := rs.ReadRune()
		if err == io.EOF || r == 0 {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if r == colon {
			break
		}
		if !(unicode.IsLetter(r) || r == hyphen) {
			return "", ErrSyntax
		}
		k.WriteRune(r)
	}
	return k.String(), nil
}

func parseValue(rs io.RuneScanner) (string, error) {
	var (
		p rune
		v bytes.Buffer
	)
	for {
		r, _, err := rs.ReadRune()
		if err == io.EOF || r == 0 {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if r == nl {
			r, _, err := rs.ReadRune()
			if err == io.EOF || r == 0 {
				break
			}
			if err != nil {
				return "", err
			}
			if !(r == space || r == tab) {
				rs.UnreadRune()
				break
			}
		}
		if r == dot && p == nl {
			continue
		}
		v.WriteRune(r)
		p = r
	}
	return v.String(), nil
}

func arch(a uint8) string {
	switch a {
	case packit.Arch32:
		return "i386"
	case packit.Arch64:
		return "amd64"
	case packit.ArchAll:
		return "all"
	default:
		return "unknown"
	}
}

func datetime(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format(debDateFormat)
}

func indent(dsc string) string {
	var body bytes.Buffer
	s := bufio.NewScanner(strings.NewReader(dsc))
	for s.Scan() {
		x := s.Text()
		if x == "" {
			io.WriteString(&body, " .\n")
		} else {
			io.WriteString(&body, " "+x+"\n")
		}
	}
	return body.String()
}

func bytesize(i int) int {
	return i >> 10
}
