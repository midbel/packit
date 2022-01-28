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
{{if .Depends }}Depends: {{join .Depends ", "}}{{end}}
{{- if .Suggests }}Suggests: {{join .Suggests ", "}}{{end}}
{{- if .Provides}}Provides: {{join .Provides ", "}}{{end}}
{{- if .Conflicts}}Conflicts: {{join .Conflicts ", "}}{{end}}
{{- if .Replaces}}Replaces: {{join .Replaces ", "}}{{end}}
Installed-Size: {{bytesize .Size}}
{{if .Compiler}}Build-Using: {{.Compiler}}{{end}}
{{if .Summary}}Description: {{.Summary}}{{end}}
{{if .Desc }}{{wrap1 (trimspace .Desc)}}{{end}}
