package cpio

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"time"
)

var magic = []byte("070701")

const trailer = "TRAILER!!!"

const (
	headerLen = 110
	fieldLen  = 8
	magicLen  = 6
)

type Header struct {
	Inode    int64
	Mode     int64
	Uid      int64
	Gid      int64
	Links    int64
	Length   int64
	Major    int64
	Minor    int64
	RMajor   int64
	RMinor   int64
	Size     int64
	Check    int64
	ModTime  time.Time
	Filename string
}

type Reader struct {
	inner *bufio.Reader
	body  io.Reader
	hdr   *Header
	err   error
}

func NewReader(r io.Reader) *Reader {
	return &Reader{inner: bufio.NewReader(r)}
}

func (r *Reader) Next() (*Header, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.body = nil

	var h Header
	if err := readMagic(r.inner); err != nil {
		r.err = err
		return nil, err
	}
	h.Inode = readHeaderField(r.inner)
	h.Mode = readHeaderField(r.inner)
	h.Uid = readHeaderField(r.inner)
	h.Gid = readHeaderField(r.inner)
	h.Links = readHeaderField(r.inner)
	h.ModTime = time.Unix(readHeaderField(r.inner), 0)
	h.Length = readHeaderField(r.inner)
	h.Major = readHeaderField(r.inner)
	h.Minor = readHeaderField(r.inner)
	h.RMajor = readHeaderField(r.inner)
	h.RMinor = readHeaderField(r.inner)
	h.Size = readHeaderField(r.inner)
	h.Check = readHeaderField(r.inner)
	h.Filename = readFilename(r.inner, h.Size)
	if mod := (headerLen + h.Size) % 4; mod != 0 {
		_, err := r.inner.Discard(4 - int(mod))
		if err != nil {
			return nil, err
		}
	}
	if h.Filename == trailer {
		return nil, io.EOF
	}
	r.hdr = &h
	n := r.hdr.Length
	if mod := n % 4; mod != 0 {
		n += 4 - mod
	}
	r.body = io.LimitReader(r.inner, n)

	return r.hdr, nil
}

func (r *Reader) Read(bs []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := io.ReadAtLeast(r.body, bs, len(bs))
	if int64(n) == r.hdr.Length {
		if mod := r.hdr.Length % 4; mod != 0 {
			io.CopyN(ioutil.Discard, r.body, 4-mod)
		}
		r.body = nil
	}
	if err != nil {
		r.err = err
	}
	return n, err
}

func readMagic(r io.Reader) error {
	bs := make([]byte, magicLen)
	if _, err := io.ReadFull(r, bs); err != nil {
		return err
	}
	if bytes.Equal(bs, magic) {
		return nil
	}
	return fmt.Errorf("unknown magic number found %x", bs)
}

func readFilename(r io.Reader, n int64) string {
	bs := make([]byte, n)
	if _, err := io.ReadFull(r, bs); err != nil {
		return ""
	}
	return string(bs[:n-1])
}

func readHeaderField(r io.Reader) int64 {
	bs := make([]byte, fieldLen)
	if _, err := io.ReadFull(r, bs); err != nil {
		return -1
	}
	i, err := strconv.ParseInt("0x"+string(bs), 0, 64)
	if err != nil {
		return -1
	}
	return i
}
