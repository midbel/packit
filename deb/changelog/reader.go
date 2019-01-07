package changelog

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/midbel/packit"
)

const (
	equal     = '='
	coma      = ','
	space     = ' '
	semicolon = ';'
	lsquare   = '<'
	rsquare   = '>'
	nl        = '\n'
)

func Parse(r io.Reader) ([]packit.Change, error) {
	var rs io.RuneScanner
	if x, ok := r.(io.RuneScanner); ok {
		rs = x
	} else {
		rs = bufio.NewReader(r)
	}
	_ = rs
	var cs []packit.Change
	return cs, nil
}

func parseTrailer(rs io.RuneScanner, c *packit.Change) error {
	if _, err := readUntil(rs, ' ', nil, nil); err != nil {
		return err
	}
	trim := func(s string) (string, error) { return strings.TrimSpace(s), nil }

	if n, err := readUntil(rs, '<', nil, trim); err != nil {
		return err
	} else {
		c.Maintainer.Name = n
	}
	if e, err := readUntil(rs, '>', nil, trim); err != nil {
		return err
	} else {
		c.Maintainer.Email = e
	}

	d, err := readUntil(rs, '\n', nil, trim)
	if err != nil {
		return err
	}
	if w, err := parseTime(d); err != nil {
		return err
	} else {
		c.When = w.UTC()
	}
	return nil
}

func parseTime(d string) (time.Time, error) {
	fs := []string{
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 -0700",
	}
	for _, f := range fs {
		if d, err := time.Parse(f, d); err == nil {
			return d, nil
		}
	}
	return time.Time{}, fmt.Errorf("can not parse date: %s", d)
}

func parseHeader(rs io.RuneScanner, c *packit.Change) error {
	if r, _, err := rs.ReadRune(); r == '\n' {
		return parseHeader(rs, c)
	} else if r == '#' {
		for r, _, _ := rs.ReadRune(); r != '\n'; r, _, _ = rs.ReadRune() {
		}
		return parseHeader(rs, c)
	} else {
		if err == io.EOF {
			return err
		}
		rs.UnreadRune()
	}
	var err error
	if c.Name, err = readUntil(rs, ' ', checkPackageRune, nil); err != nil {
		return err
	}
	if c.Version, err = readUntil(rs, ' ', checkVersionRune, nil); err != nil {
		return err
	}
	if c.Distrib, err = readList(rs, ' ', ';'); err != nil {
		return err
	}
	if _, err = readOptions(rs); err != nil {
		return err
	}
	return nil
}

func readOptions(rs io.RuneScanner) (map[string]string, error) {
	readKey := func() (string, error) {
		var key bytes.Buffer
		for r, _, err := rs.ReadRune(); r != '='; r, _, err = rs.ReadRune() {
			if r == 0 || err != nil {
				return "", fmt.Errorf("oups parsing key")
			}
			if r == ' ' && key.Len() == 0 {
				continue
			}
			if !(unicode.IsLetter(r) || r == '-') {
				return "", fmt.Errorf("invalid token in key: %c", r)
			}
			key.WriteRune(r)
		}
		return strings.TrimSpace(key.String()), nil
	}
	readValue := func() (string, bool, error) {
		var (
			value bytes.Buffer
			done  bool
		)
		for r, _, err := rs.ReadRune(); r != ','; r, _, err = rs.ReadRune() {
			if r == 0 || err != nil {
				return "", done, fmt.Errorf("oups parsing value")
			}
			if r == '\n' {
				done = true
				break
			}
			value.WriteRune(r)
		}
		return strings.TrimSpace(value.String()), done, nil
	}
	vs := make(map[string]string)
	for {
		k, err := readKey()
		if err != nil {
			return nil, err
		}
		v, done, err := readValue()
		if err != nil {
			return nil, err
		}
		vs[k] = v
		if done {
			break
		}
	}
	return vs, nil
}

func readList(rs io.RuneScanner, sep, lim rune) ([]string, error) {
	var (
		vs []string
		b  bytes.Buffer
	)
	for {
		r, _, err := rs.ReadRune()
		if err != nil {
			return nil, err
		}
		if r == 0 {
			return nil, fmt.Errorf("unexpected end of file")
		}
		if r == sep {
			vs = append(vs, b.String())
			b.Reset()
			continue
		}
		if r == lim {
			break
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		vs = append(vs, b.String())
	}
	return vs, nil
}

type checkFunc func(rune) (bool, bool)

type strFunc func(string) (string, error)

func readUntil(rs io.RuneScanner, sep rune, chk checkFunc, fn strFunc) (string, error) {
	var b bytes.Buffer
	for r, _, _ := rs.ReadRune(); r != sep; r, _, _ = rs.ReadRune() {
		if r == 0 {
			return "", fmt.Errorf("unexpected end of file")
		}
		if chk != nil {
			ok, skip := chk(r)
			if skip {
				continue
			}
			if !ok {
				return "", fmt.Errorf("illegal token: %c", r)
			}
		}
		b.WriteRune(r)
	}
	var err error

	str := b.String()
	if fn != nil {
		str, err = fn(str)
	}
	return str, err
}

func parseBody(rs io.RuneScanner, c *packit.Change) error {
	var body bytes.Buffer

	readLine := func() error {
		for r, _, _ := rs.ReadRune(); r != 0 && r != '\n'; r, _, _ = rs.ReadRune() {
			body.WriteRune(r)
		}
		body.WriteRune('\n')
		if r, _, err := rs.ReadRune(); r == 0 || err != nil {
			return fmt.Errorf("end of file???")
		} else {
			rs.UnreadRune()
		}
		return nil
	}
	for r, _, _ := rs.ReadRune(); r != 0; r, _, _ = rs.ReadRune() {
		if r == '\n' {
			continue
		}
		if r == ' ' {
			r, _, _ := rs.ReadRune()
			if r == ' ' {
				if err := readLine(); err != nil {
					return err
				}
			} else if r == '-' {
				rs.UnreadRune()
				break
			} else {
				return fmt.Errorf("invalid syntax")
			}
		}
	}
	c.Body = body.String()
	return nil
}

func checkVersionRune(k rune) (bool, bool) {
	if k == '(' || k == ')' {
		return false, true
	}
	if k == '~' {
		return true, false
	}
	return checkPackageRune(k)
}

func checkPackageRune(k rune) (bool, bool) {
	return unicode.IsLetter(k) || unicode.IsDigit(k) || k == '.' || k == '+' || k == '-', false
}

func checkPackageName(n string) error {
	r := strings.NewReader(n)
	for k, _, _ := r.ReadRune(); k != 0; k, _, _ = r.ReadRune() {
		if unicode.IsLetter(k) || unicode.IsDigit(k) || k == '.' || k == '+' || k == '-' {
			continue
		} else {
			return fmt.Errorf("%s not a valid package name", n)
		}
	}
	return nil
}
