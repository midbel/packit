package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

var ErrPattern = errors.New("invalid pattern")

func main() {
	walk := flag.Bool("w", false, "walk")
	flag.Parse()

	ignore, err := Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *walk {
		for path := range ignore.Walk(flag.Arg(1)) {
			fmt.Println(path)
		}
	} else {
		for i := 1; i < flag.NArg(); i++ {
			name := filepath.Clean(flag.Arg(i))
			name = filepath.ToSlash(name)

			ok := ignore.Match(name)
			fmt.Println(">>", name, ok)
		}
	}

}

type PatternFile struct {
	matchers []Matcher
}

func Open(file string) (*PatternFile, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var (
		scan   = bufio.NewScanner(r)
		ignore PatternFile
	)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		var (
			reverse bool
			dirOnly bool
			absOnly bool
		)
		if reverse = strings.HasPrefix(line, "!"); reverse {
			line = line[1:]
		}
		line = filepath.Clean(line)
		line = filepath.ToSlash(line)

		dirOnly = strings.HasSuffix(line, "/")
		absOnly = strings.HasPrefix(line, "/")
		_, _ = dirOnly, absOnly

		line = strings.TrimFunc(line, func(r rune) bool { return r == '/' })

		mt, err := Parse(line)
		if err != nil {
			return nil, err
		}
		if reverse {
			mt = Invert(mt)
		}
		ignore.matchers = append(ignore.matchers, mt)
	}
	return &ignore, nil
}

func (i *PatternFile) Walk(dir string) iter.Seq[string] {
	it := func(yield func(string) bool) {
		root := os.DirFS(dir)
		fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if i.Match(path) {
				if !yield(path) {
					return fs.SkipAll
				}
			}
			return nil
		})
	}
	return it
}

func (i *PatternFile) Match(file string) bool {
	for j := range i.matchers {
		if i.matchers[j].Match(file) {
			return true
		}
	}
	return false
}

func (i *PatternFile) match(rs io.RuneScanner) bool {
	return false
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
			if c, _, _ := rs.ReadRune(); c == '*' {
				if len(curr) != 0 {
					return nil, ErrPattern
				}
				if c, _, _ = rs.ReadRune(); c != '/' {
					return nil, ErrPattern
				}
				all = append(all, newSegment(nil))
				break
			}
			rs.UnreadRune()
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
		rs.UnreadRune()
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

func parseChoice2(rs io.RuneScanner, first rune) (Matcher, error) {
	ch, _, err := rs.ReadRune()
	if err != nil {
		return nil, ErrPattern
	}
	var chars []rune
	if ch == '-' {
		last, _, err := rs.ReadRune()
		if err != nil {
			return nil, ErrPattern
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
				return nil, ErrPattern
			}
			if ch == ']' {
				rs.UnreadRune()
				break
			}
			chars = append(chars, ch)
		}
	}
	if len(chars) == 0 {
		return nil, ErrPattern
	}
	return newChoice(string(chars)), nil
}

func parseChoice(rs io.RuneScanner) (Matcher, error) {
	ch, _, err := rs.ReadRune()
	if err != nil {
		return nil, ErrPattern
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
			return nil, ErrPattern
		}
		if first == ']' {
			break
		}
		mt, err := parseChoice2(rs, first)
		if err != nil {
			return nil, err
		}
		list = append(list, mt)
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

func (p pathMatcher) Match(value string) bool {
	value = filepath.Clean(value)
	value = filepath.ToSlash(value)
	return p.Match2(value)
}

func (p pathMatcher) Match2(value string) bool {
	var (
		parts  = strings.Split(value, "/")
		offset int
	)
	for i, mt := range p.matchers {
		if s, ok := mt.(segment); ok && s.any() {
			next := i + 1
			if next >= len(p.matchers) {
				return true
			}
			var result bool
			for x := range parts[offset:] {
				result = p.matchers[next].Match(parts[offset+x])
				if result {
					offset += x
					break
				}
			}
			if !result {
				return result
			}
			continue
		}
		if offset >= len(parts) {
			return false
		}
		if ok := mt.Match(parts[offset]); !ok {
			return ok
		}
		offset++
	}
	return true
}

func (p pathMatcher) match(rs io.RuneScanner) bool {
	return false
}

type drainer struct{}

func newDrainer() Matcher {
	return drainer{}
}

func (d drainer) Match(value string) bool {
	return d.match(strings.NewReader(value))
}

func (d drainer) String() string {
	return "$"
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

func (s star) String() string {
	return fmt.Sprintf("*%s", s.next)
}

func (s star) Match(value string) bool {
	return s.match(strings.NewReader(value))
}

func (s star) match(rs io.RuneScanner) bool {
	for {
		ch, _, err := rs.ReadRune()
		if err != nil {
			return s.next == nil
		}
		var valid bool
		if t, ok := s.next.(interface{ test(rune) bool }); ok {
			valid = t.test(ch)
		} else {
			valid = s.next.match(strings.NewReader(string(ch)))
		}
		if valid {
			rs.UnreadRune()
			return s.next.match(rs)
		}
	}
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

func (m multichoice) match(rs io.RuneScanner) bool {
	for i := range m.values {
		if m.values[i].match(rs) {
			return true
		}
		rs.UnreadRune()
	}
	return false
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

func (c choice) Match(value string) bool {
	return c.match(strings.NewReader(value))
}

func (c choice) String() string {
	return c.values
}

func (c choice) match(rs io.RuneScanner) bool {
	ch, _, err := rs.ReadRune()
	if err != nil {
		return false
	}
	return c.test(ch)
}

func (c choice) test(ch rune) bool {
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

func (_ any) String() string {
	return "?"
}

func (_ any) match(rs io.RuneScanner) bool {
	_, _, err := rs.ReadRune()
	if err != nil {
		return false
	}
	return true
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

func (i invert) Match(value string) bool {
	return !i.inner.Match(value)
}

func (i invert) match(rs io.RuneScanner) bool {
	return false
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

func (i literal) Match(value string) bool {
	return i.match(strings.NewReader(value))
}

func (i literal) String() string {
	return i.str
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

func (i literal) test(ch rune) bool {
	other, _ := utf8.DecodeRuneInString(i.str)
	return other == ch
}
