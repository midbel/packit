{{range .Changes}}
{{- $.Name}} ({{.Version}}) unstable; urgency=low

  {{.Summary}}

{{range .Changes}}  * {{.}}
{{end}}
 -- {{with .Maintainer}}{{.Name}}{{if .Email}} <{{.Email}}>{{end}}{{end}}  {{.When.Format "Mon, 02 Jan 2006 15:04:05 -0700"}}
{{end}}