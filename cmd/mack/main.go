package main

import (
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/midbel/cli"
)

const helpText = `{{.Name}} manages binary packages (deb, rpm).

Usage:

  {{.Name}} command [arguments]

The commands are:

{{range .Commands}}{{printf "  %-9s %s" .String .Short}}
{{end}}

Use {{.Name}} [command] -h for more information about its usage.
`

var commands = []*cli.Command{
	{
		Run:   runDeb,
		Usage: "deb <config,...>",
		Short: "create deb files from many configuration files",
	},
	{
		Run:   runShow,
		Usage: "show <config,...>",
		Short: "show information of deb files",
	},
}

func main() {
	log.SetFlags(0)
	if err := cli.Run(commands, usage, nil); err != nil {
		log.Fatalln(err)
	}
}

func usage() {
	data := struct {
		Name     string
		Commands []*cli.Command
	}{
		Name:     filepath.Base(os.Args[0]),
		Commands: commands,
	}
	t := template.Must(template.New("help").Parse(helpText))
	t.Execute(os.Stderr, data)

	os.Exit(2)
}
