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
