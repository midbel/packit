package deb

import (
  "io"

  "github.com/midbel/tape/ar"
  "github.com/midbel/mack"
)

type Package struct {

}

func (p *Package) Valid() bool {
  return false
}

func (p *Package) Control() (*mack.Control, error) {
  return nil, nil
}

func Open(r io.Reader) (*Package, error) {
  a, err := ar.NewReader(r)
  if err != nil {
    return nil, err
  }
  _ = a
  return &Package{}, nil
}
