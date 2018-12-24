package packit

import (
	"bytes"
	"io"
)

type blank struct {
	io.Writer
	last byte
}

func Clean(w io.Writer) io.Writer {
	return cleanBlank(w)
}

func cleanBlank(w io.Writer) io.Writer {
	return &blank{Writer: w}
}

func (b *blank) Write(bs []byte) (int, error) {
	var (
		xs     []byte
		offset int
	)
	if b.last != 0 {
		xs = make([]byte, len(bs)+1)
		xs[0], offset = b.last, 1
	} else {
		xs = make([]byte, len(bs))
	}
	copy(xs[offset:], bs)

	xs = bytes.Replace(xs, []byte{0x0a, 0x0a}, []byte{0x0a}, -1)
	b.last = bs[len(bs)-1]
	_, err := b.Writer.Write(xs[offset:])
	return len(bs), err
}
