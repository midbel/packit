package glob

import (
	"errors"
	"io"
	"strings"
)

func parseLine(line string) (Matcher, error) {
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
			m, err := parseRange(rs)
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
		res, err = parseRange(rs)
	case '?':
		res = newAny()
	default:
		res = parseLiteral(rs, ch)
	}
	return newStar(res), err
}

func parseRange(rs io.RuneScanner) (Matcher, error) {
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
		mt, err := parseChoice(rs, first)
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

func parseChoice(rs io.RuneScanner, first rune) (Matcher, error) {
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

func isSpecial(ch rune) bool {
	return ch == '*' || ch == '[' || ch == '/' || ch == '?'
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9')
}
