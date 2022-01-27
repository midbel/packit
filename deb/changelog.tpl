{{.Package}} ({{.Version}}); urgency=low
{{range $c := .Changes}}
* {{$c.Title}}
{{if $c.Desc}}{{wrap2 (trimspace $c.Desc)}}{{end}}

{{with $c.Maintainer}}-- {{.Name}}{{if .Email}}<{{.Email}}>{{end}}  {{datetime $c.When}}{{end}}
{{end -}}
