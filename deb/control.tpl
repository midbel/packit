Package: {{.Package}}
Version: {{.Version}}
{{with .Essential}}Essential: yes{{end}}
Section: {{.Section}}
Priority: {{if .Priority}}{{.Priority}}{{else}}optional{{end}}
Date: {{datetime .Date}}
Architecture: {{arch .Arch}}
{{with .Vendor}}Vendor: {{.}}{{end}}
{{with .Maintainer}}Maintainer: {{.Name}}{{if .Email}} <{{.Email}}>{{end}}{{end}}
{{with .Home}}Homepage: {{.}}{{end}}
{{with .Depends }}Depends: {{deplist .}}{{end}}
{{with .Suggests }}Suggests: {{deplist .}}{{end}}
{{with .Provides}}Provides: {{deplist .}}{{- else}}{{end}}
{{with .Conflicts}}Conflicts: {{deplist .}}{{end}}
{{with .Replaces}}Replaces: {{deplist .}}{{end}}
{{with .Compiler}}Build-Using: {{.}}{{end}}
Installed-Size: {{bytesize .Size}}
{{with .Summary}}Description: {{.}}
{{if $.Desc }}{{wrap1 (trimspace $.Desc)}}{{end}}
{{- end}}
