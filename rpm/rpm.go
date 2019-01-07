package rpm

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/midbel/packit"
)

func Arch(a uint8) string {
	switch a {
	default:
		return "unknown"
	case packit.Arch32:
		return "i386"
	case packit.Arch64:
		return "x86_64"
	case packit.ArchAll:
		return "noarch"
	}
}

func Build(mf *packit.Makefile) (packit.Builder, error) {
	if mf == nil {
		return nil, fmt.Errorf("empty makefile")
	}
	b := builder{
		when:    time.Now(),
		control: mf.Control,
		files:   mf.Files,
		changes: mf.Changes,
	}
	return &b, nil
}

func Open(file string) (packit.Package, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var (
		p pkg
		s *signature
	)
	if p.name, err = readLead(r); err != nil {
		return nil, err
	}
	if s, err = readSignature(r); err != nil {
		return nil, err
	}
	md, sh1, sh2 := md5.New(), sha1.New(), sha256.New()
	total := counter(0)
	rw := io.TeeReader(r, io.MultiWriter(md, sh2, &total))
	if p.control, p.history, err = readMeta(io.TeeReader(rw, sh1)); err != nil {
		return nil, err
	}
	if s.Sha1 != "" && s.Sha1 != hex.EncodeToString(sh1.Sum(nil)) {
		return nil, invalidSignature(p.name, "header", "sha1")
	}
	if p.data, err = readData(rw, p.control.Format); err != nil {
		if err != packit.ErrUnsupportedPayloadFormat {
			return nil, err
		}
	} else {
		if z := p.data.Size(); s.Payload >= 0 && int64(z) != s.Payload {
			return nil, fmt.Errorf("invalid payload size (expected %d, got %d)", s.Payload, z)
		}
		if z := total.Size(); s.Size >= 0 && z != s.Size {
			return nil, fmt.Errorf("invalid size (expected %d, got %d)", s.Size, z)
		}
		if s.MD5 != "" && s.MD5 != hex.EncodeToString(md.Sum(nil)) {
			return nil, invalidSignature(p.name, "package", "md5")
		}
		if s.Sha256 != "" && s.Sha256 != hex.EncodeToString(sh2.Sum(nil)) {
			return nil, invalidSignature(p.name, "package", "sha256")
		}
	}
	return &p, nil
}

func invalidSignature(n, w, t string) error {
	return fmt.Errorf("%s (%s): invalid signature (%s)", n, w, t)
}

type counter int64

func (c *counter) Size() int64 {
	return int64(*c)
}

func (c *counter) Write(bs []byte) (int, error) {
	n := len(bs)
	*c = counter(int64(*c) + int64(n))
	return n, nil
}

const (
	rpmFileConf         = 1 << 0
	rpmFileDoc          = 1 << 1
	rpmFileAllowMissing = 1 << 3
	rpmFileNoReplace    = 1 << 4
	rpmFileGhost        = 1 << 6
	rpmFileLicense      = 1 << 7
	rpmFileReadme       = 1 << 8
)

func fileFlags(file *packit.File) int32 {
	var f int32
	if file.Conf {
		f |= rpmFileConf
	}
	if file.Doc {
		f |= rpmFileDoc
	}
	if file.License {
		return rpmFileLicense
	}
	if file.Readme {
		return rpmFileReadme
	}
	return f
}

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
	rpmLeadLen  = 96
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
	rpmTagPayload      = 1124
	rpmTagCompressor   = 1125
	rpmTagPayloadFlags = 1126
	rpmTagFileClass    = 1141
)

const (
	rpmTagFileSizes   = 1028
	rpmTagFileModes   = 1030
	rpmTagFileDevs    = 1033
	rpmTagFileTimes   = 1034
	rpmTagFileDigests = 1035
	rpmTagFileLinks   = 1036
	rpmTagFileFlags   = 1037
	rpmTagOwners      = 1039
	rpmTagGroups      = 1040
	rpmTagFileInodes  = 1096
	rpmTagFileLangs   = 1097
	rpmTagDirIndexes  = 1116
	rpmTagBasenames   = 1117
	rpmTagDirnames    = 1118
)

const (
	rpmTagChangeTime = 1080
	rpmTagChangeName = 1081
	rpmTagChangeText = 1082
)

const (
	rpmTagFilenames = 5000
	rpmTagBugURL    = 5012
	rpmTagEncoding  = 5068
)

type fieldType uint32

func (f fieldType) String() string {
	switch f {
	default:
		return "unknown"
	case fieldNull:
		return "char"
	case fieldChar:
		return "char"
	case fieldInt8:
		return "int8"
	case fieldInt16:
		return "int16"
	case fieldInt32:
		return "int32"
	case fieldInt64:
		return "int64"
	case fieldString:
		return "string"
	case fieldBinary:
		return "binary"
	case fieldStrArray:
		return "strarray"
	case fieldI18NString:
		return "i18n"
	}
}

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
