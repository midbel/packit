package rpm

import (
  "compress/gzip"
  "io"

  "github.com/midbel/mack"
  "github.com/midbel/mack/cpio"
)

const MagicRPM = 0xedabeedb
const MagicHDR = 0x008eade8

type builder struct {
  inner io.Writer

  size int64
}

func NewBuilder(w io.Writer) mack.Builder {
  return &builder{w}
}

func (w *builder) Build(c mack.Control, files []*mack.File) error {
  return nil
}
