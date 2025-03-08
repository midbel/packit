Package: {{.Name}}
{{with .Essential}}Essential: yes{{end}}
Version: {{.Version}}
{{with .Maintainer}}Maintainer: {{.Name}}{{if .Email}} <{{.Email}}>{{end}}{{end}}
Section: {{.Section}}
Priority: {{if .Priority}}{{.Priority}}{{else}}optional{{end}}
Architecture: {{.Arch}}
{{with .Vendor}}Vendor: {{.}}{{end}}
{{with .Home}}Homepage: {{.}}{{end}}
{{if ne .BuildWith.Name ""}}Built-Using: {{.BuildWith.Name}} (={{.BuildWith.Version}}){{end}}
Installed-Size: {{fmtsize .TotalSize}}
{{with $e := .Requires}}Depends: {{range $i, $c := $e}}{{if gt $i 0 }}, {{end}}{{dependency $c}}{{end}}{{end}}
{{with $e := .Recommends}}Recommends: {{range $i, $c := $e}}{{if gt $i 0 }}, {{end}}{{dependency $c}}{{end}}{{end}}
{{with $e := .Suggests}}Suggests: {{range $i, $c := $e}}{{if gt $i 0 }}, {{end}}{{dependency $c}}{{end}}{{end}}
{{with $e := .Breaks}}Breaks: {{range $i, $c := $e}}{{if gt $i 0 }}, {{end}}{{dependency $c}}{{end}}{{end}}
{{with $e := .Conflicts}}Conflicts: {{range $i, $c := $e}}{{if gt $i 0 }}, {{end}}{{dependency $c}}{{end}}{{end}}
{{with $e := .Replaces}}Replaces: {{range $i, $c := $e}}{{if gt $i 0 }}, {{end}}{{dependency $c}}{{end}}{{end}}
{{with $e := .Enhances}}Enhances: {{range $i, $c := $e}}{{if gt $i 0 }}, {{end}}{{dependency $c}}{{end}}{{end}}
{{with $e := .Provides}}Provides: {{range $i, $c := $e}}{{if gt $i 0 }}, {{end}}{{dependency $c}}{{end}}{{end}}
{{with .Summary}}Description: {{.}}{{end}}
{{ fmtdesc .Desc }}