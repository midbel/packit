package ar

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

var (
	magic    = []byte("!<arch>")
	linefeed = []byte{0x60, 0x0A}
)

var (
	ErrMagic    = errors.New("ar: Invalid Magic")
	ErrTooShort = errors.New("ar: write too short")
	ErrTooLong  = errors.New("ar: write too long")
)

type Header struct {
	Name    string
	Uid     int
	Gid     int
	Mode    int
	Length  int
	ModTime time.Time
}

type Writer struct {
	inner io.Writer
	curr  io.Writer
	err   error
}

func NewWriter(w io.Writer) (*Writer, error) {
	if _, err := w.Write(magic); err != nil {
		return nil, err
	}
	if _, err := w.Write([]byte{linefeed[1]}); err != nil {
		return nil, err
	}
	return &Writer{inner: w}, nil
}

func (w *Writer) WriteHeader(h *Header) error {
	if w.err != nil {
		return w.err
	}
	if err := w.Flush(); err != nil {
		w.err = err
		return err
	}

	buf := new(bytes.Buffer)
	writeHeaderField(buf, path.Base(h.Name)+"/", 16)
	writeHeaderField(buf, strconv.FormatInt(h.ModTime.Unix(), 10), 12)
	writeHeaderField(buf, strconv.FormatInt(int64(h.Uid), 10), 6)
	writeHeaderField(buf, strconv.FormatInt(int64(h.Gid), 10), 6)
	writeHeaderField(buf, strconv.FormatInt(int64(h.Mode), 8), 8)
	writeHeaderField(buf, strconv.FormatInt(int64(h.Length), 10), 10)
	buf.Write(linefeed)

	_, err := io.Copy(w.inner, buf)
	if err != nil {
		w.err = err
		return err
	}
	w.curr = &fileWriter{
		writer:    w.inner,
		remaining: h.Length,
		size:      h.Length,
	}
	return nil
}

func (w *Writer) Flush() error {
	if w.curr == nil {
		return nil
	}
	if w.err != nil {
		return w.err
	}
	c := w.curr.(*fileWriter)
	if c == nil || c.remaining > 0 {
		return ErrTooShort
	}
	if mod := c.size % 2; mod == 1 {
		_, w.err = w.inner.Write(linefeed[1:])
	}
	w.curr = nil
	return w.err
}

func (w *Writer) Write(bs []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.curr.Write(bs)
	if err != nil && err != ErrTooLong {
		w.err = err
	}
	return n, err
}

func (w *Writer) Close() error {
	return w.Flush()
}

type Reader struct {
	inner *bufio.Reader
	curr  io.Reader
	err   error
}

func List(file string) ([]*Header, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := NewReader(f)
	if err != nil {
		return nil, err
	}
	var hs []*Header
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		hs = append(hs, h)
		if _, err := io.CopyN(ioutil.Discard, r, int64(h.Length)); err != nil {
			return nil, err
		}
	}
	return hs, nil
}

func NewReader(r io.Reader) (*Reader, error) {
	rs := bufio.NewReader(r)
	bs, err := rs.Peek(len(magic))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(bs, magic) {
		return nil, ErrMagic
	}
	if _, err := rs.Discard(len(bs) + 1); err != nil {
		return nil, err
	}
	return &Reader{inner: rs}, nil
}

func (r *Reader) Next() (*Header, error) {
	if r.err != nil {
		return nil, r.err
	}
	h, err := r.next()
	r.err = err
	return h, r.err
}

func (r *Reader) next() (*Header, error) {
	if r.curr != nil {
		io.Copy(ioutil.Discard, r.curr)
	}

	var h Header
	if err := readFilename(r.inner, &h); err != nil {
		r.err = err
		return nil, err
	}
	if err := readModTime(r.inner, &h); err != nil {
		return nil, err
	}
	if err := readFileInfos(r.inner, &h); err != nil {
		return nil, err
	}
	bs := make([]byte, len(linefeed))
	if _, err := r.inner.Read(bs); err != nil || !bytes.Equal(bs, linefeed) {
		return nil, err
	}
	r.curr = &fileReader{
		reader:    r.inner,
		remaining: h.Length,
		size:      h.Length,
	}
	return &h, r.err
}

func (r *Reader) Read(bs []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.curr.Read(bs)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}

func readFilename(r io.Reader, h *Header) error {
	bs, err := readHeaderField(r, 16)
	if err != nil {
		return err
	}
	h.Name = string(bs)
	return nil
}

func readModTime(r io.Reader, h *Header) error {
	bs, err := readHeaderField(r, 12)
	if err != nil {
		return err
	}
	i, err := strconv.ParseInt(string(bs), 0, 64)
	if err != nil {
		return err
	}
	h.ModTime = time.Unix(i, 0)
	return nil
}

func readFileInfos(r io.Reader, h *Header) error {
	if bs, err := readHeaderField(r, 6); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(bs), 0, 64)
		if err != nil {
			return err
		}
		h.Uid = int(i)
	}
	if bs, err := readHeaderField(r, 6); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(bs), 0, 64)
		if err != nil {
			return err
		}
		h.Gid = int(i)
	}
	if bs, err := readHeaderField(r, 8); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(bs), 0, 64)
		if err != nil {
			return err
		}
		h.Mode = int(i)
	}
	if bs, err := readHeaderField(r, 10); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(bs), 0, 64)
		if err != nil {
			return err
		}
		h.Length = int(i)
	}
	return nil
}

func readHeaderField(r io.Reader, n int) ([]byte, error) {
	bs := make([]byte, n)
	if _, err := io.ReadFull(r, bs); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(bs), nil
}

func writeHeaderField(w *bytes.Buffer, s string, n int) {
	io.WriteString(w, s)
	io.WriteString(w, strings.Repeat(" ", n-len(s)))
}

type fileReader struct {
	reader          io.Reader
	remaining, size int
}

func (f *fileReader) Read(bs []byte) (int, error) {
	if f.remaining <= 0 {
		if m := f.size % 2; f.size != 0 && m == 1 {
			b := bufio.NewReader(f.reader)
			b.ReadByte()
			f.size = 0
		}
		return 0, io.EOF
	}
	if len(bs) > f.remaining {
		bs = bs[:f.remaining]
	}
	n, err := f.reader.Read(bs)
	f.remaining -= n
	return n, err
}

type fileWriter struct {
	writer          io.Writer
	remaining, size int
}

func (f *fileWriter) Write(bs []byte) (int, error) {
	if f.remaining < 0 {
		return 0, ErrTooLong
	}
	var rest int
	switch {
	case len(bs) == 0:
		return 0, nil
	case len(bs) > f.size:
		rest = f.size
	case len(bs)-f.remaining < 0:
		rest = f.remaining
	default:
	}
	if rest > 0 {
		bs = bs[:rest]
	}
	n, err := f.writer.Write(bs)
	f.remaining -= n
	if rest > 0 {
		return n, ErrTooLong
	}
	return n, err
}
