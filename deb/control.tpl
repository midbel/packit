Package: {{.Package}}
Version: {{.Version}}
{{- if .Essential}}Essential: yes{{end}}
Section: {{.Section}}
Priority: {{if .Priority}}{{.Priority}}{{else}}optional{{end}}
Date: {{datetime .Date}}
Architecture: {{arch .Arch}}
{{if .Vendor}}Vendor: {{.Vendor}}{{end}}
{{with .Maintainer}}Maintainer: {{.Name}}{{if .Email}} <{{.Email}}>{{end}}{{end}}
{{if .Home}}Homepage: {{.Home}}{{end}}
{{if .Depends }}Depends: {{deplist .Depends}}{{end}}
{{- if .Suggests }}Suggests: {{deplist .Suggests}}{{end}}
{{- if .Provides}}Provides: {{deplist .Provides}}{{end}}
{{- if .Conflicts}}Conflicts: {{deplist .Conflicts}}{{end}}
{{- if .Replaces}}Replaces: {{deplist .Replaces}}{{end}}
Installed-Size: {{bytesize .Size}}
{{if .Compiler}}Build-Using: {{.Compiler}}{{end}}
{{if .Summary}}Description: {{.Summary}}{{end}}
{{if .Desc }}{{wrap1 (trimspace .Desc)}}{{end}}
