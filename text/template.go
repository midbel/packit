package text

import (
  "bufio"
  "io"
  "text/template"
)

func Execute(tpl *template.Template, w io.Writer, ctx interface{}) error {
  var (
    pr, pw = io.Pipe()
    scan = bufio.NewScanner(pr)
    errch = make(chan error, 1)
  )
  defer func() {
    close(errch)
    pr.Close()
  }()
  go func() {
    errch <- tpl.Execute(pw, ctx)
    pw.Close()
  }()
  for scan.Scan() {
    line := scan.Text()
    if line == "" {
      continue
    }
    io.WriteString(w, line)
    io.WriteString(w, "\n")
  }
  if err := <- errch; err != nil {
    return err
  }
  return scan.Err()
}
