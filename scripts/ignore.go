package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var ErrPattern = errors.New("invalid pattern")

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer r.Close()

	var (
		scan = bufio.NewScanner(r)
		list []Matcher
	)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		line = filepath.Clean(line)
		line = filepath.ToSlash(line)
		line = strings.TrimPrefix(line, "/")
		fmt.Println(line)

		mt, err := Parse(line)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		list = append(list, mt)
	}
	for i := 1; i < flag.NArg(); i++ {
		name := filepath.Clean(flag.Arg(i))
		name = filepath.ToSlash(name)
		fmt.Println(">>", name, len(list))
		for j := range list {
			fmt.Println(name, list[j].Match(name))
		}
	}
}

type Matcher interface {
	Match(string) bool
	match(io.RuneScanner) bool
}

func Parse(line string) (Matcher, error) {
	var (
		rs   = strings.NewReader(line)
		all  []Matcher
		curr []Matcher
	)
	for rs.Len() > 0 {
		c, _, _ := rs.ReadRune()
		switch c {
		case '*':
			m, err := parseStar(rs)
			if err != nil {
				return nil, err
			}
			curr = append(curr, m)
		case '[':
			m, err := parseChoice(rs)
			if err != nil {
				return nil, err
			}
			curr = append(curr, m)
		case '?':
			curr = append(curr, newAny())
		case '/':
			all = append(all, newSegment(curr))
			curr = curr[:0]
		default:
			m := parseLiteral(rs, c)
			curr = append(curr, m)
		}
	}
	if len(curr) > 0 {
		all = append(all, newSegment(curr))
	}
	if len(all) == 0 {
		return nil, ErrPattern
	}
	return newMatcher(all), nil
}

func isSpecial(ch rune) bool {
	return ch == '*' || ch == '[' || ch == '/' || ch == '?'
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9')
}

func parseStar(rs *strings.Reader) (Matcher, error) {
	ch, _, err := rs.ReadRune()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return nil, err
		}
		return newStar(newDrainer()), nil
	}
	var res Matcher
	switch ch {
	case '/':
		res = newDrainer()
	case '[':
		res, err = parseChoice(rs)
	case '?':
		res = newAny()
	default:
		res = parseLiteral(rs, ch)
	}
	return newStar(res), err
}

func parseChoice(rs io.RuneScanner) (Matcher, error) {
	ch, _, err := rs.ReadRune()
	if err != nil {
		return nil, err
	}
	var (
		reverse bool
		list    []Matcher
	)
	if reverse = ch == '!'; !reverse {
		rs.UnreadRune()
	}
	for {
		first, _, err := rs.ReadRune()
		if err != nil {
			return nil, err
		}
		if first == ']' {
			break
		}
		ch, _, err := rs.ReadRune()
		if err != nil {
			return nil, err
		}
		var chars []rune
		if ch == '-' {
			last, _, err := rs.ReadRune()
			if err != nil {
				return nil, err
			}
			for i := first; i <= last; i++ {
				chars = append(chars, i)
			}
		} else {
			rs.UnreadRune()
			chars = append(chars, first)
			for {
				ch, _, err := rs.ReadRune()
				if err != nil {
					return nil, err
				}
				if ch == ']' {
					rs.UnreadRune()
					break
				}
				chars = append(chars, ch)
			}
		}
		if len(chars) == 0 {
			return ErrPattern
		}
		list = append(list, newChoice(string(chars)))
	}
	if len(list) == 0 {
		return nil, ErrPattern
	}
	var res Matcher
	if len(list) == 1 {
		res = list[0]
	} else {
		res = newMultichoice(list)
	}
	if reverse {
		res = newInvert(res)
	}
	return res, nil
}

func parseLiteral(r io.RuneScanner, ch rune) Matcher {
	var chars []rune
	chars = append(chars, ch)
	for {
		c, _, err := r.ReadRune()
		if err != nil {
			break
		}
		if isSpecial(c) {
			r.UnreadRune()
			break
		}
		chars = append(chars, c)
	}
	return newLiteral(string(chars))
}

type patternMatcher struct {
	matchers []Matcher
}

func newMatcher(list []Matcher) Matcher {
	p := patternMatcher{
		matchers: slices.Clone(list),
	}
	return p
}

func (p patternMatcher) Match(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) != len(p.matchers) {
		return false
	}
	for i := range parts {
		ok := p.matchers[i].Match(parts[i])
		if !ok {
			return ok
		}
	}
	return true
}

func (p patternMatcher) match(rs io.RuneScanner) bool {
	return false
}

type drainer struct{}

func (d drainer) Match(value string) bool {
	return d.match(strings.NewReader(value))
}

func newDrainer() Matcher {
	return drainer{}
}

func (d drainer) match(rs io.RuneScanner) bool {
	for {
		_, _, err := rs.ReadRune()
		if err != nil {
			break
		}
	}
	return true
}

type star struct {
	next Matcher
}

func newStar(next Matcher) Matcher {
	s := star{
		next: next,
	}
	return s
}

func (s star) Match(value string) bool {
	return s.match(strings.NewReader(value))
}

func (s star) match(rs io.RuneScanner) bool {
	return false
}

type multichoice struct {
	values []Matcher
}

func newMultichoice(list []Matcher) Matcher {
	m := multichoice{
		values: slices.Clone(list),
	}
	return m
}

func (m multichoice) Match(value string) bool {
	return m.match(strings.NewReader(value))
}

func (m multichoice) match(rs io.RuneScanner) bool {
	for i := range m.values {
		if m.values[i].match(rs) {
			return true
		}
		rs.UnreadRune()
	}
	return false
}

type choice struct {
	values string
}

func newChoice(value string) Matcher {
	c := choice{
		values: value,
	}
	return c
}

func (c choice) Match(value string) bool {
	return c.match(strings.NewReader(value))
}

func (c choice) match(rs io.RuneScanner) bool {
	ch, _, err := rs.ReadRune()
	if err != nil {
		return false
	}
	return strings.IndexRune(c.values, ch) >= 0
}

type any struct{}

func newAny() Matcher {
	var a any
	return a
}

func (a any) Match(value string) bool {
	return a.match(strings.NewReader(value))
}

func (a any) match(rs io.RuneScanner) bool {
	_, _, err := rs.ReadRune()
	if err != nil {
		return false
	}
	return true
}

type invert struct {
	inner Matcher
}

func newInvert(mt Matcher) Matcher {
	i := invert{
		inner: mt,
	}
	return i
}

func (i invert) Match(value string) bool {
	return i.match(strings.NewReader(value))
}

func (i invert) match(rs io.RuneScanner) bool {
	return !i.inner.match(rs)
}

type segment struct {
	all []Matcher
}

func newSegment(all []Matcher) Matcher {
	s := segment{
		all: slices.Clone(all),
	}
	return s
}

func (s segment) Match(value string) bool {
	rs := strings.NewReader(value)
	return s.match(rs)
}

func (s segment) match(r io.RuneScanner) bool {
	for i := range s.all {
		if !s.all[i].match(r) {
			return false
		}
	}
	return true
}

type literal struct {
	str string
}

func newLiteral(str string) Matcher {
	i := literal{
		str: str,
	}
	return i
}

func (i literal) Match(value string) bool {
	return i.match(strings.NewReader(value))
}

func (i literal) match(rs io.RuneScanner) bool {
	if n, ok := rs.(interface{ Len() int }); ok {
		if n.Len() != len(i.str) {
			return false
		}
	}
	values := []rune(i.str)
	for j := range values {
		ch, _, err := rs.ReadRune()
		if err != nil {
			return false
		}
		if ch != values[j] {
			return false
		}
	}
	return true
}
