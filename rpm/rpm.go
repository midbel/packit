package rpm

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/mack"
	"github.com/midbel/tape"
	"github.com/midbel/tape/cpio"
)

const (
	SigTypeRPM = 5
	MajorRPM   = 3
	MinorRPM   = 0
	MagicRPM   = 0xedabeedb
	MagicHDR   = 0x008eade8
)

const (
	TagSignatureIndex = 62
	TagImmutableIndex = 63
)

const (
	SigBase    = 256
	SigDSA     = SigBase + 11
	SigRSA     = SigBase + 12
	SigSha1    = SigBase + 13
	SigSha256  = SigBase + 17
	SigLength  = 1000
	SigPGP     = 1002
	SigMD5     = 1004
	SigPayload = 1007
)

const (
	TagPackage   = 1000
	TagVersion   = 1001
	TagRelease   = 1002
	TagSummary   = 1004
	TagDesc      = 1005
	TagBuildTime = 1006
	TagSize      = 1009
	TagVendor    = 1011
	TagLicense   = 1014
	TagPackager  = 1015
	TagURL       = 1020
	TagOS        = 1021
	TagArch      = 1022
	TagSizes     = 1028
	TagModes     = 1030
	TagDigests   = 1035
	TagContribs  = 1081
	TagBasenames = 1117
	TagDirnames  = 1118
	TagOwners    = 1039
	TagGroups    = 1040
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
	size, body, err := w.writeArchive(files)
	if err != nil {
		return err
	}
	meta, err := writeMetadata(&c, w.filenames, w.md5sums, w.sizes)
	if err != nil {
		return err
	}

	var data bytes.Buffer
	md5sum, sha1sum, sha256sum := md5.New(), sha1.New(), sha256.New()
	if _, err := io.Copy(io.MultiWriter(&data, md5sum, sha1sum, sha256sum), io.MultiReader(meta, body)); err != nil {
		return err
	}
	fs := []Field{
		number{tag: SigLength, kind: int32(Int32), Value: int64(data.Len())},
		number{tag: SigPayload, kind: int32(Int32), Value: int64(size)},
		binarray{tag: SigMD5, Value: md5sum.Sum(nil)},
		binarray{tag: SigSha1, Value: sha1sum.Sum(nil)},
		binarray{tag: SigSha256, Value: sha256sum.Sum(nil)},
	}
	sig, err := writeFields(fs, TagSignatureIndex, true)
	if err != nil {
		return nil
	}
	_, err = io.Copy(w.inner, io.MultiReader(sig, &data))
	return err
}

func writeFields(fs []Field, tag int32, pad bool) (*bytes.Buffer, error) {
	sort.Slice(fs, func(i, j int) bool {
		return fs[i].Tag() < fs[j].Tag()
	})
	var meta, body, store bytes.Buffer
	for _, f := range fs {
		writeField(&body, &store, f)
	}

	binary.Write(&meta, binary.BigEndian, uint32((MagicHDR<<8)|1))
	binary.Write(&meta, binary.BigEndian, uint32(0))
	binary.Write(&meta, binary.BigEndian, int32(len(fs)+1))
	binary.Write(&meta, binary.BigEndian, int32(store.Len()+16))

	f := index{tag: tag, Value: -16 * int32(1+len(fs))}
	writeField(&meta, &store, f)

	_, err := io.Copy(&meta, io.MultiReader(&body, &store))
	if mod := meta.Len() % 8; mod != 0 && pad {
		bs := make([]byte, 8-mod)
		meta.Write(bs)
	}
	return &meta, err
}

func writeField(b, s *bytes.Buffer, f Field) {
	binary.Write(b, binary.BigEndian, f.Tag())
	binary.Write(b, binary.BigEndian, f.Type())
	binary.Write(b, binary.BigEndian, int32(s.Len()))
	binary.Write(b, binary.BigEndian, f.Len())

	s.Write(f.Bytes())
}

func (w *builder) writeArchive(files []*mack.File) (int, *bytes.Buffer, error) {
	body := new(bytes.Buffer)
	ark := cpio.NewWriter(body)
	for _, f := range files {
		n, bs, err := writeFile(ark, f)
		if err != nil {
			return 0, nil, err
		}
		w.sizes = append(w.sizes, n)
		w.md5sums = append(w.md5sums, fmt.Sprintf("%x", bs))
		w.filenames = append(w.filenames, f.String())
	}
	if err := ark.Close(); err != nil {
		return 0, nil, err
	}
	total := body.Len()

	bz := new(bytes.Buffer)
	gz, _ := gzip.NewWriterLevel(bz, gzip.BestCompression)
	if _, err := io.Copy(gz, body); err != nil {
		return total, nil, err
	}
	return total, bz, nil
}

func writeFile(w *cpio.Writer, f *mack.File) (int64, []byte, error) {
	r, err := os.Open(f.Src)
	if err != nil {
		return 0, nil, err
	}
	defer r.Close()
	i, err := r.Stat()
	if err != nil {
		return 0, nil, err
	}
	h := tape.Header{
		Filename: f.String(),
		Mode:     int64(i.Mode()),
		Length:   i.Size(),
		ModTime:  i.ModTime(),
	}
	if err := w.WriteHeader(&h); err != nil {
		return h.Length, nil, err
	}
	s := md5.New()
	if _, err := io.Copy(w, io.TeeReader(r, s)); err != nil {
		return h.Length, nil, err
	}
	return h.Length, s.Sum(nil), err
}

type Field interface {
	Tag() int32
	Type() int32
	Len() int32
	Skip() bool
	Bytes() []byte
}

type binarray struct {
	tag   int32
	Value []byte
}

func (b binarray) Skip() bool    { return len(b.Value) == 0 }
func (b binarray) Tag() int32    { return b.tag }
func (b binarray) Type() int32   { return int32(Binary) }
func (b binarray) Len() int32    { return int32(len(b.Value)) }
func (b binarray) Bytes() []byte { return b.Value }

type number struct {
	tag   int32
	kind  int32
	Value int64
}

func (n number) Skip() bool  { return false }
func (n number) Tag() int32  { return n.tag }
func (n number) Type() int32 { return n.kind }
func (n number) Len() int32  { return 1 }
func (n number) Bytes() []byte {
	var w bytes.Buffer
	switch EntryType(n.kind) {
	case Int8:
		binary.Write(&w, binary.BigEndian, int8(n.Value))
	case Int16:
		binary.Write(&w, binary.BigEndian, int16(n.Value))
	case Int32:
		binary.Write(&w, binary.BigEndian, int32(n.Value))
	case Int64:
		binary.Write(&w, binary.BigEndian, int64(n.Value))
	}
	return w.Bytes()
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

type array struct {
	tag   int32
	Value []string
}

func (a array) Skip() bool  { return len(a.Value) == 0 }
func (a array) Tag() int32  { return a.tag }
func (a array) Type() int32 { return int32(StringArray) }
func (a array) Len() int32  { return int32(len(a.Value)) }
func (a array) Bytes() []byte {
	var b bytes.Buffer
	for _, v := range a.Value {
		io.WriteString(&b, v)
		b.WriteByte(0)
	}
	return b.Bytes()
}

type index struct {
	tag   int32
	Value int32
}

func (i index) Skip() bool  { return false }
func (i index) Tag() int32  { return i.tag }
func (i index) Type() int32 { return int32(Binary) }
func (i index) Len() int32  { return 16 }
func (i index) Bytes() []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, i.tag)
	binary.Write(&b, binary.BigEndian, int32(Binary))
	binary.Write(&b, binary.BigEndian, i.Value)
	binary.Write(&b, binary.BigEndian, i.Len())
	return b.Bytes()
}

func controlToFields(c *mack.Control) []Field {
	var fs []Field

	fs = append(fs, varchar{tag: TagPackage, Value: c.Package})
	fs = append(fs, varchar{tag: TagVersion, Value: c.Version})
	fs = append(fs, varchar{tag: TagSummary, Value: c.Summary})
	fs = append(fs, number{tag: TagBuildTime, kind: int32(Int32), Value: time.Now().Unix()})
	fs = append(fs, varchar{tag: TagDesc, Value: c.Desc})
	fs = append(fs, varchar{tag: TagVendor, Value: c.Vendor})
	fs = append(fs, varchar{tag: TagPackager, Value: c.Maintainer.String()})
	fs = append(fs, varchar{tag: TagLicense, Value: c.License})
	fs = append(fs, varchar{tag: TagURL, Value: c.Home})
	fs = append(fs, varchar{tag: TagOS, Value: "linux"})
	fs = append(fs, varchar{tag: TagOS, Value: c.Arch})

	if n := len(c.Contributors); n > 0 {
		cs := make([]string, n)
		for i, c := range c.Contributors {
			cs[i] = c.String()
		}
		fs = append(fs, array{tag: TagContribs, Value: cs})
	}

	return fs
}

const (
	defaultUser  = "root"
	defaultGroup = "root"
)

func writeMetadata(c *mack.Control, files, sums []string, sizes []int64) (*bytes.Buffer, error) {
	fs := controlToFields(c)
	ds, bs := make([]string, len(files)), make([]string, len(files))
	os, gs := make([]string, len(files)), make([]string, len(files))
	for i := range files {
		d, n := filepath.Split(files[i])
		if !strings.HasPrefix(d, "/") {
			d = "/" + d
		}
		ds[i], bs[i] = d, n
		os[i], gs[i] = defaultUser, defaultGroup
	}
	zs := make([]string, len(sizes))
	var total int64
	for i := range sizes {
		zs[i] = strconv.FormatInt(sizes[i], 10)
		total += sizes[i]
	}
	fs = append(fs, number{tag: TagSize, kind: int32(Int32), Value: total})
	fs = append(fs, array{tag: TagBasenames, Value: bs})
	fs = append(fs, array{tag: TagDirnames, Value: ds})
	fs = append(fs, array{tag: TagOwners, Value: os})
	fs = append(fs, array{tag: TagGroups, Value: gs})
	fs = append(fs, array{tag: TagDigests, Value: sums})
	fs = append(fs, array{tag: TagSizes, Value: zs})
	return writeFields(fs, TagImmutableIndex, false)
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
