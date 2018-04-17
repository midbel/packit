package rpm

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/midbel/mack"
	"github.com/midbel/mack/cpio"
)

const (
	SigTypeRPM = 5
	MajorRPM   = 3
	MinorRPM   = 0
	MagicRPM   = 0xedabeedb
	MagicHDR   = 0x008eade8
)

type builder struct {
	inner io.Writer

	md5sums   []string
	filenames []string
	sizes     []int64
}

func NewBuilder(w io.Writer) mack.Builder {
	return &builder{inner: w}
}

func (w *builder) Build(c mack.Control, files []*mack.File) error {
	e := Lead{
		Major:   MajorRPM,
		Minor:   MinorRPM,
		SigType: SigTypeRPM,
		Name:    c.Package,
	}
	if err := writeLead(w.inner, e); err != nil {
		return err
	}
	meta, err := writeMetadata(&c)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w.inner, meta); err != nil {
		return err
	}
	body, err := w.writeArchive(files)
	if err != nil {
		return err
	}
	_, err = io.Copy(w.inner, body)
	return err
}

func (w *builder) writeArchive(files []*mack.File) (*bytes.Buffer, error) {
	body := new(bytes.Buffer)
	ark := cpio.NewWriter(body)
	for _, f := range files {
		bs, err := writeFile(ark, f)
		if err != nil {
			return nil, err
		}
		w.sizes = append(w.sizes, int64(len(bs)))
		w.md5sums = append(w.md5sums, fmt.Sprintf("%x", bs))
		w.filenames = append(w.filenames, f.String())
	}
	if err := ark.Close(); err != nil {
		return nil, err
	}
	bz := new(bytes.Buffer)
	gz, _ := gzip.NewWriterLevel(bz, gzip.BestCompression)
	if _, err := io.Copy(gz, body); err != nil {
		return nil, err
	}
	return bz, nil
}

func writeFile(w *cpio.Writer, f *mack.File) ([]byte, error) {
	r, err := os.Open(f.Src)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	i, err := r.Stat()
	if err != nil {
		return nil, err
	}
	stat, ok := i.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return nil, fmt.Errorf("can not get stat for info %s", f)
	}
	h := cpio.Header{
		Filename: f.String(),
		Mode:     int64(i.Mode()),
		Length:   i.Size(),
		ModTime:  i.ModTime(),
		Major:    int64(stat.Dev >> 32),
		Minor:    int64(stat.Dev & 0xFFFFFFFF),
	}
	if err := w.WriteHeader(&h); err != nil {
		return nil, err
	}
	s := md5.New()
	if _, err := io.Copy(io.MultiWriter(w, s), r); err != nil {
		return nil, err
	}
	return s.Sum(nil), err
}

const (
	TagPackage = 1000
	TagVersion = 1001
	TagRelease = 1002
	TagSummary = 1004
	TagDesc    = 1005
	TagVendor  = 1011
	TagLicense = 1014
	TagURL     = 1020
)

type Field interface {
	Tag() int32
	Type() int32
	Len() int32
	Skip() bool
	Bytes() []byte
}

type number struct {
	tag   int32
	kind  int32
	Value int64
}

func (n number) Skip() bool  { return false }
func (n number) Tag() int32  { return n.tag }
func (n number) Type() int32 { return int32(n.kind) }
func (n number) Len() int32 {
	switch e := n.kind; EntryType(e) {
	case Int8:
		return 1
	case Int16:
		return 2
	case Int32:
		return 4
	default:
		return 8
	}
}
func (n number) Bytes() []byte {
	return nil
}

type varchar struct {
	tag   int32
	Value string
}

func (v varchar) Skip() bool  { return len(v.Value) == 0 }
func (v varchar) Tag() int32  { return v.tag }
func (v varchar) Type() int32 { return int32(String) }
func (v varchar) Len() int32  { return 1 }
func (v varchar) Bytes() []byte {
	return append([]byte(v.Value), 0)
}

func controlToFields(c *mack.Control) []Field {
	var fs []Field

	fs = append(fs, varchar{tag: TagPackage, Value: c.Package})
	fs = append(fs, varchar{tag: TagVersion, Value: c.Version})
	fs = append(fs, varchar{tag: TagSummary, Value: c.Summary})
	fs = append(fs, varchar{tag: TagDesc, Value: c.Desc})
	fs = append(fs, varchar{tag: TagVendor, Value: c.Vendor})
	fs = append(fs, varchar{tag: TagLicense, Value: c.License})
	fs = append(fs, varchar{tag: TagURL, Value: c.Home})

	return fs
}

func writeMetadata(c *mack.Control) (*bytes.Buffer, error) {
	fs := controlToFields(c)
	var meta, body, store bytes.Buffer

	for _, f := range fs {
		if f.Skip() {
			continue
		}
		offset := int32(store.Len())
		binary.Write(&body, binary.BigEndian, f.Tag())
		binary.Write(&body, binary.BigEndian, f.Type())
		binary.Write(&body, binary.BigEndian, offset)
		binary.Write(&body, binary.BigEndian, f.Len())
		store.Write(f.Bytes())
	}
	binary.Write(&meta, binary.BigEndian, uint32((MagicHDR<<8) | 1))
	binary.Write(&meta, binary.BigEndian, int32(len(fs)))
	binary.Write(&meta, binary.BigEndian, int32(body.Len()))

	_, err := io.Copy(&meta, io.MultiReader(&body, &store))
	return &meta, err
}

func writeLead(w io.Writer, e Lead) error {
	body := new(bytes.Buffer)
	binary.Write(body, binary.BigEndian, uint32(MagicRPM))
	binary.Write(body, binary.BigEndian, e.Major)
	binary.Write(body, binary.BigEndian, e.Minor)
	binary.Write(body, binary.BigEndian, e.Type)
	binary.Write(body, binary.BigEndian, e.Arch)
	if n := e.Name; len(n) > 66 {
		io.WriteString(body, n[:66])
	} else {
		bs := make([]byte, 66-len(n))
		vs := append([]byte(n), bs...)
		body.Write(vs)
	}

	binary.Write(body, binary.BigEndian, e.Os)
	binary.Write(body, binary.BigEndian, e.SigType)
	for i := 0; i < 4; i++ {
		binary.Write(body, binary.BigEndian, uint32(0))
	}

	_, err := io.Copy(w, body)
	return err
}
