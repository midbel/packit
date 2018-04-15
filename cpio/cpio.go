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

var (
	magicASCII = []byte("070701")
	magicCRC   = []byte("070702")
)

const trailer = "TRAILER!!!"

const (
	blockSize = 512
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
	Check    int64
	ModTime  time.Time
	Filename string
}

type Writer struct {
	inner  io.Writer
	blocks int64
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{inner: w}
}

func (w *Writer) WriteHeader(h *Header) error {
	return w.writeHeader(h, false)
}

func (w *Writer) Write(bs []byte) (int, error) {
	vs := make([]byte, len(bs))
	copy(vs, bs)
	if mod := len(bs) % 4; mod != 0 {
		zs := make([]byte, 4-mod)
		vs = append(vs, zs...)
	}
	n, err := w.inner.Write(vs)
	if err != nil {
		return n, err
	}
	w.blocks += int64(n)
	return len(bs), err
}

func (w *Writer) Close() error {
	h := Header{Filename: trailer}
	if err := w.writeHeader(&h, true); err != nil {
		return err
	}
	var err error
	if mod := w.blocks % blockSize; mod != 0 {
		bs := make([]byte, blockSize-mod)
		_, err = w.inner.Write(bs)
	}
	return err
}

func (w *Writer) writeHeader(h *Header, trailing bool) error {
	buf := new(bytes.Buffer)
	z := int64(len(h.Filename)) + 1

	buf.Write(magicASCII)
	writeHeaderInt(buf, h.Inode)
	writeHeaderInt(buf, h.Mode)
	writeHeaderInt(buf, h.Uid)
	writeHeaderInt(buf, h.Gid)
	writeHeaderInt(buf, h.Links)
	if t := h.ModTime; t.IsZero() {
		writeHeaderInt(buf, 0)
	} else {
		writeHeaderInt(buf, t.Unix())
	}
	writeHeaderInt(buf, h.Length)
	writeHeaderInt(buf, h.Major)
	writeHeaderInt(buf, h.Minor)
	writeHeaderInt(buf, h.RMajor)
	writeHeaderInt(buf, h.RMinor)
	writeHeaderInt(buf, z)
	writeHeaderInt(buf, 0)
	writeFilename(buf, h.Filename)

	w.blocks += headerLen + z
	if mod := (headerLen + z) % 4; mod != 0 && !trailing {
		bs := make([]byte, 4-mod)
		n, _ := buf.Write(bs)
		w.blocks += int64(n)
	}

	_, err := io.Copy(w.inner, buf)
	return err
}

type Reader struct {
	inner  *bufio.Reader
	body   io.Reader
	hdr    *Header
	err    error
	remain int
}

func NewReader(r io.Reader) *Reader {
	return &Reader{inner: bufio.NewReader(r)}
}

func (r *Reader) Next() (*Header, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.body = nil

	var (
		h Header
		z int64
	)
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
	z = readHeaderField(r.inner)
	h.Check = readHeaderField(r.inner)
	h.Filename = readFilename(r.inner, z)
	if mod := (headerLen + z) % 4; mod != 0 {
		_, err := r.inner.Discard(4 - int(mod))
		if err != nil {
			return nil, err
		}
	}
	if h.Filename == trailer {
		return &h, io.EOF
	}
	r.hdr = &h
	n := r.hdr.Length
	if mod := n % 4; mod != 0 {
		n += 4 - mod
	}
	r.body, r.remain = io.LimitReader(r.inner, n), int(r.hdr.Length)

	return r.hdr, nil
}

func (r *Reader) Read(bs []byte) (int, error) {
	if r.body == nil {
		return 0, io.EOF
	}
	if r.err != nil {
		return 0, r.err
	}
	n, err := io.ReadAtLeast(r.body, bs, len(bs))
	r.remain -= n
	if r.remain <= 0 {
		if mod := r.hdr.Length % 4; mod != 0 {
			_, e := io.CopyN(ioutil.Discard, r.body, 4-mod)
			if e != nil {
				err = e
			}
		}
		r.body, r.remain = nil, 0
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
	if bytes.Equal(bs, magicCRC) || bytes.Equal(bs, magicASCII) {
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
	//TODO: check for rewrite with fmt.Fscanf()
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

func writeHeaderInt(w *bytes.Buffer, f int64) {
	fmt.Fprintf(w, "%08x", uint64(f))
}

func writeFilename(w *bytes.Buffer, f string) {
	io.WriteString(w, f+"\x00")
}
