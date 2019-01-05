package rw

import (
	"bytes"
	"io"
	"strings"
)

const (
	MinLineLength = 72
	MaxLineLength = 80
)

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
	var body, line bytes.Buffer
	for {
		r, _, err := r.ReadRune()
		if err == io.EOF || r == 0 {
			break
		}
		if err != nil {
			return "", err
		}
		if (r == ' ' && line.Len() >= min) || r == '\n' {
			line.WriteRune('\n')
			io.Copy(&body, &line)
		} else {
			line.WriteRune(r)
		}
	}
	if line.Len() > 0 {
		io.Copy(&body, &line)
	}
	return body.String(), nil
}
