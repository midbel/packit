package packit

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/tape"
	"github.com/midbel/tape/cpio"
)

var (
	rpmMagic  = []byte{0xed, 0xab, 0xee, 0xdb}
	rpmHeader = []byte{0x8e, 0xad, 0xe8, 0x01}
)

const (
	rpmMajor    = 3
	rpmMinor    = 0
	rpmBinary   = 0
	rpmSigType  = 5
	rpmEntryLen = 16
)

const (
	rpmTagSignatureIndex = 62
	rpmTagImmutableIndex = 63
)

const (
	rpmPayloadFormat     = "cpio"
	rpmPayloadCompressor = "gzip"
	rpmPayloadFlags      = "9"
)

const (
	rpmSigBase = 256
	// rpmSigPGP     = 1002
	// rpmSigDSA     = rpmSigBase + 11
	// rpmSigRSA     = rpmSigBase + 12
	rpmSigSha1    = rpmSigBase + 13
	rpmSigSha256  = rpmSigBase + 17
	rpmSigLength  = 1000
	rpmSigMD5     = 1004
	rpmSigPayload = 1007
)

const (
	rpmTagPackage      = 1000
	rpmTagVersion      = 1001
	rpmTagRelease      = 1002
	rpmTagSummary      = 1004
	rpmTagDesc         = 1005
	rpmTagBuildTime    = 1006
	rpmTagBuildHost    = 1007
	rpmTagSize         = 1009
	rpmTagDistrib      = 1010
	rpmTagVendor       = 1011
	rpmTagLicense      = 1014
	rpmTagPackager     = 1015
	rpmTagGroup        = 1016
	rpmTagURL          = 1020
	rpmTagOS           = 1021
	rpmTagArch         = 1022
	rpmTagSizes        = 1028
	rpmTagModes        = 1030
	rpmTagDigests      = 1035
	rpmTagChangeTime   = 1080
	rpmTagChangeName   = 1081
	rpmTagChangeText   = 1082
	rpmTagBasenames    = 1117
	rpmTagDirnames     = 1118
	rpmTagPayload      = 1124
	rpmTagCompressor   = 1125
	rpmTagPayloadFlags = 1126
	rpmTagOwners       = 1039
	rpmTagGroups       = 1040
)

type RPM struct {
	*Makefile
}

func (r rpmHeaderEntry) Size() int64 {
	z := (r.Count * rpmEntryLen) + r.Len
	return int64(z)
}

func openRPM(r io.Reader) error {
	// step 1: read lead: check major/minor version
	if err := readLead(r); err != nil {
		return err
	}
	// step 2: read signature
	if _, err := readSignature(r); err != nil {
		return err
	}
	// step 3: read metadata
	h1x := sha1.New()
	if _, err := readHeader(io.TeeReader(r, h1x)); err != nil {
		return err
	}
	// step 4: payload??? ignore for now
	return nil
}

func readSignature(r io.Reader) (*Signature, error) {
	var s Signature
	f := func(tag int32, v interface{}) error {
		switch tag {
		case rpmSigSha1:
			s.Sha1 = v.(string)
		case rpmSigSha256:
			if xs, ok := v.([]byte); ok {
				s.Sha256 = hex.EncodeToString(xs)
			}
		case rpmSigMD5:
			if xs, ok := v.([]byte); ok {
				s.MD5 = hex.EncodeToString(xs)
			}
		case rpmSigLength:
			s.Size = v.(int64)
		case rpmSigPayload:
			s.Payload = v.(int64)
		default:
		}
		return nil
	}
	return &s, readFields(r, true, f)
}

func readHeader(r io.Reader) (*Makefile, error) {
	var m Makefile
	f := func(tag int32, v interface{}) error {
		return nil
	}
	return &m, readFields(r, false, f)
}

func readFields(r io.Reader, padding bool, f func(int32, interface{}) error) error {
	if f == nil {
		f = func(_ int32, _ interface{}) error { return nil }
	}
	var e rpmHeaderEntry
	if err := binary.Read(r, binary.BigEndian, &e); err != nil {
		return err
	}
	magic := binary.BigEndian.Uint32(rpmHeader) >> 8
	if e.Magic >> 8 != magic {
		return fmt.Errorf("invalid RPM header: %06x", e.Magic)
	}
	if v := e.Magic & 0xFF; byte(v) != rpmHeader[3] {
		return fmt.Errorf("unsupported RPM header version: %d", v)
	}
	es := make([]rpmEntry, int(e.Count))
	for i := 0; i < len(es); i++ {
		if err := binary.Read(r, binary.BigEndian, &es[i]); err != nil {
			return err
		}
	}
	size := e.Len
	if m := (e.Len+rpmEntryLen+(e.Count*rpmEntryLen))%8; padding && m > 0 {
		size += 8-m
	}
	xs := make([]byte, int(size))
	if _, err := io.ReadFull(r, xs); err != nil {
		return err
	}
	stor := bytes.NewReader(xs)
	sort.Slice(es, func(i, j int) bool { return es[i].Offset < es[j].Offset })
	for i := 0; i < len(es); i++ {
		e := es[i]
		if _, err := stor.Seek(int64(e.Offset), io.SeekStart); err != nil {
			return err
		}
		n := stor.Len()
		if j := i+1; j < len(es) {
			n = int(es[j].Offset-es[i].Offset)
		}
		v, err := e.Decode(io.LimitReader(stor, int64(n)))
		if err != nil {
			return err
		}
		if err := f(e.Tag, v); err != nil {
			return err
		}
	}
	return nil
}

type rpmHeaderEntry struct {
	Magic uint32
	Spare uint32
	Count int32
	Len   int32
}

type rpmEntry struct {
	Tag    int32
	Type   fieldType
	Offset int32
	Len    int32
}

func (e rpmEntry) Decode(r io.Reader) (interface{}, error) {
	var (
		v interface{}
		err error
	)
	switch e.Type {
	case fieldChar:
		var i byte
		err, v = binary.Read(r, binary.BigEndian, &i), i
	case fieldInt8:
		var i int8
		err, v = binary.Read(r, binary.BigEndian, &i), int64(i)
	case fieldInt16:
		var i int16
		err, v = binary.Read(r, binary.BigEndian, &i), int64(i)
	case fieldInt32:
		var i int32
		err, v = binary.Read(r, binary.BigEndian, &i), int64(i)
	case fieldInt64:
		var i int64
		err, v = binary.Read(r, binary.BigEndian, &i), i
	case fieldString, fieldI18NString:
		s := bufio.NewScanner(r)
		s.Split(nullSplit)
		if s.Scan() {
			v = s.Text()
		}
		err = s.Err()
	case fieldStrArray:
		s := bufio.NewScanner(r)
		s.Split(nullSplit)

		vs := make([]string, int(e.Len))
		for i := 0; i < len(vs); i++ {
			if b := s.Scan(); b {
				vs[i] = s.Text()
			}
		}
		v, err = vs, s.Err()
	case fieldBinary:
		xs := make([]byte, int(e.Len))
		if _, err = io.ReadFull(r, xs); err == nil {
			v = xs
		}
	default:
		err = fmt.Errorf("unknown field type %d", e.Type)
	}
	return v, err
}

func nullSplit(bs []byte, ateof bool) (int, []byte, error) {
	if ix := bytes.IndexByte(bs, 0); ix < 0 {
		return 0, nil, nil
	} else {
		xs := make([]byte, ix)
		return copy(xs, bs)+1, xs, nil
	}
}

func readLead(r io.Reader) error {
	c := struct {
		Magic     uint32
		Major     uint8
		Minor     uint8
		Type      uint16
		Arch      uint16
		Name      [66]byte
		Os        uint16
		Signature uint16
		Spare     [16]byte
	}{}
	if err := binary.Read(r, binary.BigEndian, &c); err != nil {
		return err
	}
	if c.Magic != binary.BigEndian.Uint32(rpmMagic) {
		return fmt.Errorf("invalid RPM magic: %08x", c.Magic)
	}
	if c.Major != rpmMajor {
		return fmt.Errorf("unsupported RPM version: %d.%d", c.Major, c.Minor)
	}
	if c.Signature != rpmSigType {
		return fmt.Errorf("invalid RPM signature type: %d", c.Signature)
	}
	return nil
}

func (r *RPM) PackageName() string {
	return r.Control.PackageName() + "." + rpmArch(r.Control.Arch) + ".rpm"
}

func (r *RPM) Build(w io.Writer) error {
	if err := r.writeLead(w); err != nil {
		return err
	}
	var data, meta bytes.Buffer
	size, err := r.writeData(&data)
	if err != nil {
		return err
	}

	sh1 := sha1.New()
	if err := r.writeHeader(io.MultiWriter(&meta, sh1)); err != nil {
		return err
	}

	md, sh256 := md5.New(), sha256.New()
	var body bytes.Buffer
	if _, err := io.Copy(io.MultiWriter(&body, md, sh256), io.MultiReader(&meta, &data)); err != nil {
		return err
	}
	var sig bytes.Buffer
	if err := r.writeSums(io.MultiWriter(w, &sig), size, body.Len(), md, sh1, sh256); err != nil {
		return err
	}

	_, err = io.Copy(w, &body)
	return err
}

func (r *RPM) writeSums(w io.Writer, data, all int, md, h1, h256 hash.Hash) error {
	h1x := h1.Sum(nil)
	h2x := h256.Sum(nil)
	mdx := md.Sum(nil)

	fields := []rpmField{
		number{tag: rpmSigLength, kind: fieldInt32, Value: int64(all)},
		number{tag: rpmSigPayload, kind: fieldInt32, Value: int64(data)},
		varchar{tag: rpmSigSha1, Value: hex.EncodeToString(h1x[:])},
		binarray{tag: rpmSigMD5, Value: mdx[:]},
		binarray{tag: rpmSigSha256, Value: h2x[:]},
	}
	return writeFields(w, fields, rpmTagSignatureIndex, true)
}

func (r *RPM) writeHeader(w io.Writer) error {
	fields := r.controlToFields()
	fields = append(fields, r.filesToFields()...)

	return writeFields(w, fields, rpmTagImmutableIndex, false)
}

func (r *RPM) writeData(w io.Writer) (int, error) {
	var data bytes.Buffer
	wc := cpio.NewWriter(&data)

	digest := md5.New()
	for _, i := range r.Files {
		f, err := os.Open(i.Src)
		if err != nil {
			return 0, err
		}
		var (
			size int64
			r    io.Reader
		)
		if i.Compress {
			var body bytes.Buffer
			z := gzip.NewWriter(&body)
			if _, err := io.Copy(z, f); err != nil {
				return 0, err
			}
			if err := z.Close(); err != nil {
				return 0, err
			}
			r, size = &body, int64(body.Len())
		} else {
			s, err := f.Stat()
			if err != nil {
				return 0, err
			}
			size, r = s.Size(), f
		}
		h := tape.Header{
			Filename: i.String(),
			Mode:     int64(i.Mode()),
			Length:   size,
			Uid:      0,
			Gid:      0,
			ModTime:  time.Now().Truncate(time.Minute),
		}
		if err := wc.WriteHeader(&h); err != nil {
			return 0, err
		}
		if i.Size, err = io.Copy(io.MultiWriter(wc, digest), r); err != nil {
			return 0, err
		}
		i.Sum = fmt.Sprintf("%x", digest.Sum(nil))

		f.Close()
		digest.Reset()
	}
	if err := wc.Close(); err != nil {
		return 0, err
	}
	size := data.Len()
	z, _ := gzip.NewWriterLevel(w, gzip.BestCompression)
	if _, err := io.Copy(z, &data); err != nil {
		return 0, err
	}
	if err := z.Close(); err != nil {
		return 0, err
	}
	return size, nil
}

func (r *RPM) writeLead(w io.Writer) error {
	body := make([]byte, 96)
	copy(body, rpmMagic)
	// binary.BigEndian.PutUint32(body[0:], uint32(rpmMagic))
	binary.BigEndian.PutUint16(body[4:], uint16(rpmMajor)<<8|uint16(rpmMinor))
	binary.BigEndian.PutUint16(body[6:], rpmBinary)
	binary.BigEndian.PutUint16(body[8:], 0)
	if n := []byte(r.Control.PackageName()); len(n) <= 65 {
		copy(body[10:], n)
	} else {
		copy(body[10:], n[:65])
	}
	binary.BigEndian.PutUint16(body[76:], 1)
	binary.BigEndian.PutUint16(body[78:], rpmSigType)

	_, err := w.Write(body)
	return err
}

func (r *RPM) controlToFields() []rpmField {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = defaultHost
	}
	when := time.Now().UTC().Truncate(time.Minute)
	var fs []rpmField
	fs = append(fs, varchar{tag: rpmTagPackage, Value: r.Control.Package})
	fs = append(fs, varchar{tag: rpmTagVersion, Value: r.Control.Version})
	fs = append(fs, varchar{tag: rpmTagRelease, Value: r.Control.Release})
	fs = append(fs, varchar{tag: rpmTagSummary, kind: fieldI18NString, Value: r.Control.Summary})
	fs = append(fs, varchar{tag: rpmTagDesc, kind: fieldI18NString, Value: r.Control.Desc})
	fs = append(fs, varchar{tag: rpmTagGroup, kind: fieldI18NString, Value: r.Control.Section})
	fs = append(fs, varchar{tag: rpmTagOS, Value: r.Control.Os})
	fs = append(fs, number{tag: rpmTagBuildTime, kind: fieldInt32, Value: when.Unix()})
	fs = append(fs, varchar{tag: rpmTagBuildHost, Value: host})
	fs = append(fs, varchar{tag: rpmTagDistrib, Value: r.Control.Vendor})
	fs = append(fs, varchar{tag: rpmTagVendor, Value: r.Control.Vendor})
	fs = append(fs, varchar{tag: rpmTagPackager, Value: "packit"})
	fs = append(fs, varchar{tag: rpmTagLicense, Value: r.Control.License})
	fs = append(fs, varchar{tag: rpmTagURL, Value: r.Control.Home})
	fs = append(fs, varchar{tag: rpmTagOS, Value: r.Control.Os})
	fs = append(fs, varchar{tag: rpmTagArch, Value: rpmArch(r.Control.Arch)})
	fs = append(fs, varchar{tag: rpmTagPayload, Value: rpmPayloadFormat})
	fs = append(fs, varchar{tag: rpmTagCompressor, Value: rpmPayloadCompressor})
	fs = append(fs, varchar{tag: rpmTagPayloadFlags, Value: rpmPayloadFlags})

	if n := len(r.Changes); n > 0 {
		ts, cs, ls := make([]int64, n), make([]string, n), make([]string, n)
		m := r.Control.Maintainer
		for i := range r.Changes {
			ts[i] = r.Changes[i].When.Unix()
			if r.Changes[i].Maintainer == nil {
				cs[i] = m.String()
			} else {
				cs[i] = r.Changes[i].Maintainer.String()
			}
			ls[i] = strings.Join(r.Changes[i].Changes, "\n")
		}
		fs = append(fs, numarray{tag: rpmTagChangeTime, kind: fieldInt32, Value: ts})
		fs = append(fs, strarray{tag: rpmTagChangeName, Values: cs})
		fs = append(fs, strarray{tag: rpmTagChangeText, Values: ls})
	}
	return fs
}

func (r *RPM) filesToFields() []rpmField {
	var fs []rpmField

	z := len(r.Files)
	dirs, bases := make([]string, z), make([]string, z)
	users, groups := make([]string, z), make([]string, z)
	sizes, digests := make([]string, z), make([]string, z)
	for i := range r.Files {
		d, n := filepath.Split(r.Files[i].String())
		if !strings.HasPrefix(d, "/") {
			d = "/" + d
		}
		dirs[i], bases[i] = d, n
		users[i], groups[i] = defaultUser, defaultGroup
		sizes[i], digests[i] = strconv.FormatInt(r.Files[i].Size, 10), r.Files[i].Sum

		r.Size += r.Files[i].Size
	}

	fs = append(fs, number{tag: rpmTagSize, kind: fieldInt32, Value: r.Size})
	fs = append(fs, strarray{tag: rpmTagBasenames, Values: bases})
	fs = append(fs, strarray{tag: rpmTagDirnames, Values: dirs})
	fs = append(fs, strarray{tag: rpmTagOwners, Values: users})
	fs = append(fs, strarray{tag: rpmTagGroups, Values: groups})
	fs = append(fs, strarray{tag: rpmTagDigests, Values: digests})
	fs = append(fs, strarray{tag: rpmTagSizes, Values: sizes})

	return fs
}

func writeFields(w io.Writer, fields []rpmField, tag int32, pad bool) error {
	var (
		hdr, idx, stor bytes.Buffer
		count          int32
	)

	writeField := func(f rpmField) {
		var lim int
		switch e := f.Type(); e {
		case fieldInt8:
			lim = 1
		case fieldInt16:
			lim = 2
		case fieldInt32:
			lim = 4
		case fieldInt64:
			lim = 8
		}
		if lim > 0 {
			if mod := stor.Len() % lim; mod != 0 {
				for i := 0; i < lim-mod; i++ {
					stor.WriteByte(0)
				}
			}
		}
		binary.Write(&hdr, binary.BigEndian, f.Tag())
		binary.Write(&hdr, binary.BigEndian, f.Type())
		binary.Write(&hdr, binary.BigEndian, int32(stor.Len()))
		binary.Write(&hdr, binary.BigEndian, f.Len())
		stor.Write(f.Bytes())
	}

	for i := range fields {
		if fields[i].Skip() {
			continue
		}
		writeField(fields[i])
		count++
	}
	if tag > 0 {
		count++
		binary.Write(&idx, binary.BigEndian, uint32(tag))
		binary.Write(&idx, binary.BigEndian, uint32(fieldBinary))
		binary.Write(&idx, binary.BigEndian, int32(stor.Len()))
		binary.Write(&idx, binary.BigEndian, int32(rpmEntryLen))

		binary.Write(&stor, binary.BigEndian, uint32(tag))
		binary.Write(&stor, binary.BigEndian, uint32(fieldBinary))
		binary.Write(&stor, binary.BigEndian, int32(-hdr.Len()-rpmEntryLen))
		binary.Write(&stor, binary.BigEndian, int32(rpmEntryLen))
	}

	if _, err := w.Write(rpmHeader); err != nil {
		return err
	}
	// binary.Write(w, binary.BigEndian, uint32(rpmHeader))
	binary.Write(w, binary.BigEndian, uint32(0))
	binary.Write(w, binary.BigEndian, count)
	binary.Write(w, binary.BigEndian, int32(stor.Len()))

	n, err := io.Copy(w, io.MultiReader(&idx, &hdr, &stor))
	if m := n % 8; m != 0 && pad {
		w.Write(make([]byte, 8-m))
	}
	return err
}

type fieldType uint32

const (
	fieldNull fieldType = iota
	fieldChar
	fieldInt8
	fieldInt16
	fieldInt32
	fieldInt64
	fieldString
	fieldBinary
	fieldStrArray
	fieldI18NString
)

type rpmField interface {
	Tag() int32
	Type() fieldType
	Len() int32
	Skip() bool
	Bytes() []byte
}

type binarray struct {
	tag   int32
	Value []byte
}

func (b binarray) Skip() bool      { return len(b.Value) == 0 }
func (b binarray) Tag() int32      { return b.tag }
func (b binarray) Type() fieldType { return fieldBinary }
func (b binarray) Len() int32      { return int32(len(b.Value)) }
func (b binarray) Bytes() []byte   { return b.Value }

type numarray struct {
	tag   int32
	kind  fieldType
	Value []int64
}

func (n numarray) Skip() bool      { return len(n.Value) == 0 }
func (n numarray) Tag() int32      { return n.tag }
func (n numarray) Type() fieldType { return n.kind }
func (n numarray) Len() int32      { return int32(len(n.Value)) }
func (n numarray) Bytes() []byte {
	var w bytes.Buffer
	for _, v := range n.Value {
		writeNumber(&w, n.kind, v)
	}
	return w.Bytes()
}

type number struct {
	tag   int32
	kind  fieldType
	Value int64
}

func (n number) Skip() bool      { return false }
func (n number) Tag() int32      { return n.tag }
func (n number) Type() fieldType { return n.kind }
func (n number) Len() int32      { return 1 }
func (n number) Bytes() []byte {
	var w bytes.Buffer
	writeNumber(&w, n.kind, n.Value)
	return w.Bytes()
}

func writeNumber(w io.Writer, t fieldType, n int64) {
	switch t {
	case fieldInt8:
		binary.Write(w, binary.BigEndian, int8(n))
	case fieldInt16:
		binary.Write(w, binary.BigEndian, int16(n))
	case fieldInt32:
		binary.Write(w, binary.BigEndian, int32(n))
	case fieldInt64:
		binary.Write(w, binary.BigEndian, int64(n))
	}
}

type varchar struct {
	tag   int32
	kind  fieldType
	Value string
}

func (v varchar) Skip() bool { return len(v.Value) == 0 }
func (v varchar) Tag() int32 { return v.tag }
func (v varchar) Type() fieldType {
	if v.kind == 0 {
		return fieldString
	}
	return v.kind
}
func (v varchar) Len() int32 { return 1 }
func (v varchar) Bytes() []byte {
	return append([]byte(v.Value), 0)
}

type strarray struct {
	tag    int32
	Values []string
}

func (a strarray) Skip() bool      { return len(a.Values) == 0 }
func (a strarray) Tag() int32      { return a.tag }
func (a strarray) Type() fieldType { return fieldStrArray }
func (a strarray) Len() int32      { return int32(len(a.Values)) }
func (a strarray) Bytes() []byte {
	var b bytes.Buffer
	for _, v := range a.Values {
		io.WriteString(&b, v)
		b.WriteByte(0)
	}
	return b.Bytes()
}

type index struct {
	tag   int32
	Value int32
}

func (i index) Skip() bool      { return false }
func (i index) Tag() int32      { return i.tag }
func (i index) Type() fieldType { return fieldBinary }
func (i index) Len() int32      { return 16 }
func (i index) Bytes() []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, i.tag)
	binary.Write(&b, binary.BigEndian, fieldBinary)
	binary.Write(&b, binary.BigEndian, i.Value)
	binary.Write(&b, binary.BigEndian, i.Len())
	return b.Bytes()
}

func rpmArch(a uint8) string {
	switch a {
	case 32:
		return "i386"
	case 64:
		return "x86_64"
	default:
		return "noarch"
	}
}
