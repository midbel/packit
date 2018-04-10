package main

import (
	"flag"
	"log"
	"os"

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
		Location    string `toml:"location"`
		deb.Control `toml:"control"`
	}{}
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
}
