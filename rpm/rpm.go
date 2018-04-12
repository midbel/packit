package rpm

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
)

type RPMEntryType uint32

const (
	NullType RPMEntryType = iota
	CharType
	Int8Type
	Int16Type
	Int32Type
	Int64Type
	StringType
	BinType
	ArrayType
)

type RPMPackageType int16

func (r RPMPackageType) String() string {
	switch r {
	case 0:
		return "binary"
	case 1:
		return "source"
	default:
		return "unknown"
	}
}

const (
	Binary RPMPackageType = iota
	Source
)

const (
	RPMMagic       = 0xedabeedb
	RPMHeaderMagic = 0x8eade8
)

type Lead struct {
	Magic     uint32
	Major     uint8
	Minor     uint8
	Type      RPMPackageType
	Arch      int16
	Os        int16
	Signature int16
	Name      [66]byte
	Spare     [16]byte
}

func (l *Lead) Version() string {
	return fmt.Sprintf("%d.%d", l.Major, l.Minor)
}

func (l *Lead) String() string {
	bs := bytes.Trim(l.Name[:], "\x00")
	return string(bs)
}

type Header struct {
	Preamble uint32
	Spare    uint32
	Count    uint32
	Length   uint32
	Index    []*Entry
}

func (h Header) Magic() uint32 {
	return h.Preamble >> 8
}

func (h Header) Version() uint8 {
	v := h.Preamble & 0xFF
	return uint8(v)
}

type Entry struct {
	Tag    int32
	Type   RPMEntryType
	Offset int32
	Count  int32

	value interface{}
}

func (e Entry) String() string {
	return fmt.Sprint(e.value)
}

func (e *Entry) extract(r io.Reader) error {
	readString := func(r *bufio.Reader) (string, error) {
		bs := make([]byte, 0, 64)
		for b, err := r.ReadByte(); b != 0; b, err = r.ReadByte() {
			if err != nil {
				return "", err
			}
			bs = append(bs, b)
		}
		return string(bs), nil
	}
	switch e.Type {
	case NullType:
	case CharType:
	case Int8Type:
		var v int8
		binary.Read(r, binary.BigEndian, &v)
		e.value = v
	case Int16Type:
		var v int16
		binary.Read(r, binary.BigEndian, &v)
		e.value = v
	case Int32Type:
		var v int32
		binary.Read(r, binary.BigEndian, &v)
		e.value = v
	case Int64Type:
		var v int64
		binary.Read(r, binary.BigEndian, &v)
		e.value = v
	case StringType:
		v, err := readString(bufio.NewReader(r))
		if err != nil {
			return err
		}
		e.value = v
	case BinType:
		bs := make([]byte, int(e.Count))
		if _, err := io.ReadFull(r, bs); err != nil {
			return err
		}
		e.value = bs
	case ArrayType:
		vs := make([]string, e.Count)
		rs := bufio.NewReader(r)
		for i := range vs {
			v, err := readString(rs)
			if err != nil {
				return err
			}
			vs[i] = v
		}
		e.value = vs
	}
	return nil
}

type Package struct {
	Lead    *Lead
	Headers []*Header
}

func (p Package) Signature() Header {
	return *p.Headers[0]
}

func Open(file string) (*Package, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReader(f)

	var pkg Package
	pkg.Lead, err = readLead(r)
	if err != nil {
		return nil, err
	}

	vs := make([]byte, 4)
	binary.BigEndian.PutUint32(vs, uint32(RPMHeaderMagic))
	for {
		bs, err := r.Peek(3)
		if err != nil {
			return nil, err
		}
		fmt.Printf("%x %x\n", bs, vs[1:])
		if !bytes.Equal(bs, vs[1:]) {
			break
		}
		h, err := readHeader(r)
		if err != nil {
			return nil, err
		}
		pkg.Headers = append(pkg.Headers, h)
	}
	return &pkg, nil
}

func readHeader(r io.Reader) (*Header, error) {
	var h Header

	binary.Read(r, binary.BigEndian, &h.Preamble)
	if h.Magic() != uint32(RPMHeaderMagic) {
		return nil, fmt.Errorf("invalid magic number for signature %x", h.Magic())
	}
	binary.Read(r, binary.BigEndian, &h.Spare)
	binary.Read(r, binary.BigEndian, &h.Count)
	binary.Read(r, binary.BigEndian, &h.Length)

	h.Index = make([]*Entry, h.Count)
	for i := range h.Index {
		var e Entry
		binary.Read(r, binary.BigEndian, &e.Tag)
		binary.Read(r, binary.BigEndian, &e.Type)
		binary.Read(r, binary.BigEndian, &e.Offset)
		binary.Read(r, binary.BigEndian, &e.Count)

		h.Index[i] = &e
	}
	sort.Slice(h.Index, func(i, j int) bool {
		return h.Index[i].Offset < h.Index[j].Offset
	})
	for _, e := range h.Index {
		if err := e.extract(r); err != nil {
			return nil, err
		}
	}
	for i, rs := 1, bufio.NewReader(r); ; i++ {
		bs, err := rs.Peek(i)
		if err != nil {
			return nil, err
		}
		vs := make([]byte, i)
		if !bytes.Equal(vs, bs) {
			rs.Discard(i-1)
			break
		}
	}
	return &h, nil
}

func readLead(r io.Reader) (*Lead, error) {
	var l Lead
	binary.Read(r, binary.BigEndian, &l.Magic)
	if l.Magic != uint32(RPMMagic) {
		return nil, fmt.Errorf("not a rpm")
	}
	binary.Read(r, binary.BigEndian, &l.Major)
	binary.Read(r, binary.BigEndian, &l.Minor)
	binary.Read(r, binary.BigEndian, &l.Type)
	if !(l.Type == Binary || l.Type == Source) {
		return nil, fmt.Errorf("unknown package type")
	}
	binary.Read(r, binary.BigEndian, &l.Arch)
	if _, err := io.ReadFull(r, l.Name[:]); err != nil {
		return nil, err
	}
	binary.Read(r, binary.BigEndian, &l.Os)
	binary.Read(r, binary.BigEndian, &l.Signature)
	if _, err := io.ReadFull(r, l.Spare[:]); err != nil {
		return nil, err
	}

	return &l, nil
}
