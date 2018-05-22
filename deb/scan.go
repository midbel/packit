package deb

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const eof = rune(0)

type scanner struct {
	buffer []byte
	offset int
}

func scan(r io.Reader) *scanner {
	var (
		s scanner
		w bytes.Buffer
	)
	if n, err := io.Copy(&w, r); err == nil {
		s.buffer = make([]byte, n)
		io.ReadFull(&w, s.buffer)
	}
	return &s
}

func (s *scanner) Scan() (string, string, error) {
	var (
		k, v string
		err  error
	)
	k, err = s.scanKey()
	if err != nil {
		return k, v, err
	}
	v, err = s.scanValue()
	if err != nil {
		return k, v, err
	}
	return strings.TrimSpace(k), strings.TrimSpace(v), err
}

func (s *scanner) scanRune() rune {
	r, z := utf8.DecodeRune(s.buffer[s.offset:])
	if r == utf8.RuneError {
		return eof
	}
	s.offset += z
	return r
}

func (s *scanner) peekRune() rune {
	r, _ := utf8.DecodeRune(s.buffer[s.offset:])
	if r == utf8.RuneError {
		return eof
	}
	return r
}

func (s *scanner) scanValue() (string, error) {
	var w bytes.Buffer
	for {
		r := s.scanRune()
		if r == '\n' && s.peekRune() != ' ' {
			break
		}
		w.WriteRune(r)
	}
	return w.String(), nil
}

func (s *scanner) scanKey() (string, error) {
	r := s.scanRune()
	if r == eof {
		return "", io.EOF
	}
	if !isIdent(r) {
		return "", fmt.Errorf("invalid syntax")
	}
	var w bytes.Buffer
	w.WriteRune(r)
	for {
		r = s.scanRune()
		if r == eof {
			return "", fmt.Errorf("unexpected end of file")
		}
		if r == ':' {
			break
		}
		if !(isIdent(r) || r == '-') {
			return "", fmt.Errorf("invalid token %c", r)
		}
		w.WriteRune(r)
	}
	return w.String(), nil
}

func isIdent(r rune) bool {
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n'
}
