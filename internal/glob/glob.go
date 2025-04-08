package glob

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

var (
	ErrIgnore  = errors.New("ignore")
	ErrPattern = errors.New("invalid pattern")
)

type Matcher interface {
	Match(string) error
}

type all struct{}

func (_ all) Match(_ string) error {
	return nil
}

func Default() Matcher {
	return all{}
}

type matcherSet struct {
	patterns []Matcher
}

func Parse(file string) (Matcher, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var (
		scan = bufio.NewScanner(r)
		set  matcherSet
	)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		var reverse bool
		if reverse = strings.HasPrefix(line, "!"); reverse {
			line = line[1:]
		}
		line = filepath.Clean(line)
		line = filepath.ToSlash(line)

		line = strings.TrimFunc(line, func(r rune) bool { return r == '/' })

		mt, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		if reverse {
			mt = Invert(mt)
		}
		set.patterns = append(set.patterns, mt)

	}
	return set, nil
}

func (m matcherSet) Match(file string) error {
	for i := range m.patterns {
		err := m.patterns[i].Match(file)
		if err == nil {
			return nil
		}
	}
	return ErrIgnore
}

type pathMatcher struct {
	matchers []Matcher
}

func newMatcher(list []Matcher) Matcher {
	p := pathMatcher{
		matchers: slices.Clone(list),
	}
	return p
}

func (p pathMatcher) String() string {
	var w strings.Builder
	for i := range p.matchers {
		if i > 0 {
			w.WriteString("/")
		}
		str, ok := p.matchers[i].(fmt.Stringer)
		if ok {
			w.WriteString(str.String())
		} else {
			w.WriteString("@")
		}
	}
	return fmt.Sprintf("path(%s)", w.String())
}

func (p pathMatcher) Match(value string) error {
	value = filepath.Clean(value)
	value = filepath.ToSlash(value)

	var (
		parts  = strings.Split(value, "/")
		offset int
	)
	for i, mt := range p.matchers {
		if s, ok := mt.(segment); ok && s.any() {
			next := i + 1
			if next >= len(p.matchers) {
				return nil
			}
			var result error
			for x := range parts[offset:] {
				result = p.matchers[next].Match(parts[offset+x])
				if result == nil {
					offset += x
					break
				}
			}
			if result != nil {
				return ErrIgnore
			}
			continue
		}
		if offset >= len(parts) {
			return ErrIgnore
		}
		if err := mt.Match(parts[offset]); err != nil {
			return ErrIgnore
		}
		offset++
	}
	return nil
}

type drainer struct{}

func newDrainer() Matcher {
	return drainer{}
}

func (d drainer) Match(value string) error {
	return d.match(strings.NewReader(value))
}

func (d drainer) String() string {
	return "$"
}

func (d drainer) match(rs io.RuneScanner) error {
	for {
		_, _, err := rs.ReadRune()
		if err != nil {
			break
		}
	}
	return nil
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

func (s star) String() string {
	return fmt.Sprintf("*%s", s.next)
}

func (s star) Match(value string) error {
	return s.match(strings.NewReader(value))
}

func (s star) match(rs io.RuneScanner) error {
	for {
		ch, _, err := rs.ReadRune()
		if err != nil {
			if s.next == nil {
				return nil
			}
			return ErrIgnore
		}
		var valid bool
		if t, ok := s.next.(interface{ test(rune) bool }); ok {
			valid = t.test(ch)
		} else {
			err := s.next.match(strings.NewReader(string(ch)))
			if err != nil {
				return err
			}
			valid = true
		}
		if valid {
			rs.UnreadRune()
			if err := s.next.match(rs); err == nil {
				return nil
			}
			return ErrIgnore
		}
	}
	return ErrIgnore
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

func (m multichoice) String() string {
	var w strings.Builder
	w.WriteRune('[')
	for i := range m.values {
		str, ok := m.values[i].(fmt.Stringer)
		if ok {
			w.WriteString(str.String())
		} else {
			w.WriteString("@")
		}
	}
	w.WriteRune(']')
	return w.String()
}

func (m multichoice) Match(value string) error {
	return m.match(strings.NewReader(value))
}

func (m multichoice) match(rs io.RuneScanner) error {
	for i := range m.values {
		if err := m.values[i].match(rs); err == nil {
			return nil
		}
		rs.UnreadRune()
	}
	return ErrIgnore
}

func (m multichoice) test(ch rune) bool {
	for i := range m.values {
		t, ok := m.values[i].(interface{ test(rune) bool })
		if ok && t.test(ch) {
			return true
		}
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

func (c choice) String() string {
	return c.values
}

func (c choice) Match(value string) error {
	return c.match(strings.NewReader(value))
}

func (c choice) match(rs io.RuneScanner) error {
	ch, _, err := rs.ReadRune()
	if err != nil {
		return ErrIgnore
	}
	if c.test(ch) {
		return nil
	}
	return ErrIgnore
}

func (c choice) test(ch rune) bool {
	return strings.IndexRune(c.values, ch) >= 0
}

type any struct{}

func newAny() Matcher {
	var a any
	return a
}

func (a any) Match(value string) error {
	return a.match(strings.NewReader(value))
}

func (_ any) String() string {
	return "?"
}

func (_ any) match(rs io.RuneScanner) error {
	_, _, err := rs.ReadRune()
	if err != nil {
		return ErrIgnore
	}
	return nil
}

func (_ any) test(_ rune) bool {
	return true
}

type invert struct {
	inner Matcher
}

func Invert(mt Matcher) Matcher {
	return newInvert(mt)
}

func newInvert(mt Matcher) Matcher {
	i := invert{
		inner: mt,
	}
	return i
}

func (i invert) String() string {
	return fmt.Sprintf("not(%s)", i.inner)
}

func (i invert) Match(value string) error {
	return i.inner.Match(value)
}

func (i invert) match(rs io.RuneScanner) error {
	return nil
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

func (s segment) String() string {
	if len(s.all) == 0 {
		return "**"
	}
	var w strings.Builder
	for i := range s.all {
		str, ok := s.all[i].(fmt.Stringer)
		if ok {
			w.WriteString(str.String())
		} else {
			w.WriteString("@")
		}
	}
	return w.String()
}

func (s segment) Match(value string) error {
	rs := strings.NewReader(value)
	return s.match(rs)
}

func (s segment) match(r io.RuneScanner) error {
	for i := range s.all {
		if err := s.all[i].match(r); err != nil {
			return ErrIgnore
		}
	}
	return nil
}

func (s segment) any() bool {
	return len(s.all) == 0
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

func (i literal) String() string {
	return i.str
}

func (i literal) Match(value string) error {
	return i.match(strings.NewReader(value))
}

func (i literal) match(rs io.RuneScanner) error {
	if n, ok := rs.(interface{ Len() int }); ok {
		if n.Len() != len(i.str) {
			return ErrIgnore
		}
	}
	values := []rune(i.str)
	for j := range values {
		ch, _, err := rs.ReadRune()
		if err != nil {
			return ErrIgnore
		}
		if ch != values[j] {
			return ErrIgnore
		}
	}
	return nil
}

func (i literal) test(ch rune) bool {
	other, _ := utf8.DecodeRuneInString(i.str)
	return other == ch
}
