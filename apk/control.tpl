pkgname = {{.Package}}
pkgver = {{.Version}}
pkgdesc = {{.Summary}}
url = {{.Home }}
builddate = {{.Date.Unix}}
{{with .Maintainer}}packager = {{.Name}}{{if .Email}} <{{.Email}}>{{end}}
{{end}}
size = {{.Size}}
arch = {{.Arch}}
origin = {{.Package}}
license = {{.License}}
{{range .Depends}}depend = {{.Name}}
{{end}}
{{range .Replaces}}replace = {{.Name}}
{{end}}
{{range .Provides}}provide = {{.Name}}
{{end}}
datahash = {{.DataHash}}
