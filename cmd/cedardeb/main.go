package main

import (
	"flag"
	"log"
	"os"

	"github.com/midbel/cedar"
	"github.com/midbel/cedar/deb"
	"github.com/midbel/toml"
)

func main() {
	flag.Parse()
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()
	c := struct {
		Location string        `toml:"location"`
		Control  deb.Control   `toml:"control"`
		Files    []*cedar.File `toml:"resource"`
	}{}
	c.Control.Maintainer = cedar.Maintainer {
		Name: os.Getenv("CEDAR_MAINTAINER"),
		Email: os.Getenv("CEDAR_EMAIL"),
	}
	if err := toml.NewDecoder(f).Decode(&c); err != nil {
		log.Fatalln(err)
	}
	d, err := os.Create(c.Location)
	if err != nil {
		log.Fatalln(err)
	}
	defer d.Close()
	pkg, err := deb.NewWriter(d)
	if err != nil {
		log.Fatalln(err)
	}
	if err := pkg.WriteControl(c.Control); err != nil {
		log.Fatalln("failed to write control:", err)
	}
	for _, f := range c.Files {
		if err := pkg.WriteFile(f); err != nil {
			log.Fatalln("failed to write file:", f.Dst, err)
		}
	}
	if err := pkg.Close(); err != nil {
		os.Remove(c.Location)
		log.Fatalln(err)
	}
}
