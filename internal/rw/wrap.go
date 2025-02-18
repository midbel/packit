package rw

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

const (
	MinLineLength = 72
	MaxLineLength = 80
)

func CleanString(text string) string {
	s := bufio.NewScanner(strings.NewReader(text))
	var str bytes.Buffer
	for s.Scan() {
		if t := s.Text(); len(t) > 0 {
			io.WriteString(&str, t+"\n")
		}
	}
	if err := s.Err(); err != nil {
		return text
	}
	return strings.TrimSuffix(str.String(), "\n")
}

func CleanAndWrapDefault(text string) string {
	return WrapDefault(CleanString(text))
}

func CleanAndWrap(text string, min, max int) string {
	return Wrap(CleanString(text), min, max)
}

func WrapDefault(text string) string {
	return Wrap(text, MinLineLength, MaxLineLength)
}

func Wrap(text string, min, max int) string {
	r := strings.NewReader(text)
	w, err := wrapText(r, min, max)
	if err != nil {
		return text
	}
	return w
}

func wrapText(r io.RuneReader, min, max int) (string, error) {
	var (
		body bytes.Buffer
		line bytes.Buffer
		last int
	)
	for {
		r, _, err := r.ReadRune()
		if err == io.EOF || r == 0 {
			break
		}
		if err != nil {
			return "", err
		}
		if (r == ' ' && line.Len() >= min) || r == '\n' {
			if max > min && line.Len() >= max {
				io.CopyN(&body, &line, int64(last))
			} else {
				io.Copy(&body, &line)
			}
			last = 0
			body.WriteRune('\n')
		} else {
			line.WriteRune(r)
			if r == ' ' {
				last = line.Len()
			}
		}
	}
	if line.Len() > 0 {
		io.Copy(&body, &line)
	}
	return body.String(), nil
}
