package rpm

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
)

type EntryType int32

const (
	Null EntryType = iota
	Char
	Int8
	Int16
	Int32
	Int64
	String
	Binary
	StringArray
	I18NString
)

func (e EntryType) String() string {
	switch e {
	case Null:
		return "null"
	case Char:
		return "char"
	case Int8:
		return "int8"
	case Int16:
		return "int16"
	case Int32:
		return "int32"
	case Int64:
		return "int64"
	case String:
		return "string"
	case Binary:
		return "binary"
	case StringArray:
		return "array"
	case I18NString:
		return "i18n"
	default:
		return "unknown"
	}
}

type Lead struct {
	Major   uint8
	Minor   uint8
	Type    int16
	Arch    int16
	Os      int16
	SigType int16
	Name    string
}

type Header struct {
	Version int8
	Count   uint32
	Length  uint32
	Index   []*Entry
	store   *bytes.Buffer
}

type Entry struct {
	Tag    int32
	Type   EntryType
	Offset int32
	Count  int32
	Value  interface{}
}

func parseHeader(r *bufio.Reader) (*Header, error) {
	var (
		h Header
		m uint32
	)
	binary.Read(r, binary.BigEndian, &m)
	if m := m >> 8; m != MagicHDR {
		return nil, fmt.Errorf("invalid magic header %x", m)
	}
	h.Version = int8(m & 0xFF)
	binary.Read(r, binary.BigEndian, &m)
	binary.Read(r, binary.BigEndian, &h.Count)
	binary.Read(r, binary.BigEndian, &h.Length)

	buf, size, offset := new(bytes.Buffer), h.Length+(h.Count*16), h.Count*16
	if _, err := io.CopyN(buf, r, int64(size)); err != nil {
		return nil, err
	}
	rs := bytes.NewReader(buf.Bytes())
	buf.Reset()

	h.Index = make([]*Entry, h.Count)
	for i := range h.Index {
		var e Entry
		binary.Read(rs, binary.BigEndian, &e.Tag)
		binary.Read(rs, binary.BigEndian, &e.Type)
		binary.Read(rs, binary.BigEndian, &e.Offset)
		binary.Read(rs, binary.BigEndian, &e.Count)

		rs.Seek(int64(offset)+int64(e.Offset), io.SeekStart)
		if err := parseValue(&e, rs); err != nil {
			return nil, err
		}
		rs.Seek(int64(i*16), io.SeekStart)

		h.Index[i] = &e
	}
	sort.Slice(h.Index, func(i, j int) bool {
		return h.Index[i].Offset < h.Index[j].Offset
	})
	if mod := h.Length % 8; mod != 0 {
		r.Discard(8 - int(mod))
	}
	return &h, nil
}

func parseLead(r *bufio.Reader) (*Lead, error) {
	var (
		e Lead
		m uint32
	)
	binary.Read(r, binary.BigEndian, &m)
	if m != MagicRPM {
		return nil, fmt.Errorf("invalid magic rpm %x", m)
	}
	binary.Read(r, binary.BigEndian, &e.Major)
	binary.Read(r, binary.BigEndian, &e.Minor)
	binary.Read(r, binary.BigEndian, &e.Type)
	binary.Read(r, binary.BigEndian, &e.Arch)
	if !(e.Type == 0 || e.Type == 1) {
		return nil, fmt.Errorf("invalid package type")
	}
	bs := make([]byte, 66)
	if _, err := io.ReadFull(r, bs); err != nil {
		return nil, err
	} else {
		e.Name = string(bytes.Trim(bs, "\x00"))
	}
	binary.Read(r, binary.BigEndian, &e.Os)
	binary.Read(r, binary.BigEndian, &e.SigType)
	if _, err := io.ReadFull(r, bs[:16]); err != nil {
		return nil, err
	}
	return &e, nil
}

func parseValue(e *Entry, r *bytes.Reader) error {
	switch e.Type {
	case Null:
	case Char:
	case Int8:
		var v int8
		binary.Read(r, binary.BigEndian, &v)
		e.Value = v
	case Int16:
		var v int16
		binary.Read(r, binary.BigEndian, &v)
		e.Value = v
	case Int32:
		var v int32
		binary.Read(r, binary.BigEndian, &v)
		e.Value = v
	case Int64:
		var v int64
		binary.Read(r, binary.BigEndian, &v)
		e.Value = v
	case String:
		v, err := readString(r)
		if err != nil {
			return err
		}
		e.Value = v
	case Binary:
		bs := make([]byte, e.Count)
		if _, err := io.ReadFull(r, bs); err != nil {
			return err
		}
		e.Value = fmt.Sprintf("%x", bs)
	case StringArray:
		vs := make([]string, e.Count)
		for i := range vs {
			v, err := readString(r)
			if err != nil {
				return err
			}
			vs[i] = v
		}
		e.Value = vs
	case I18NString:
	default:
		return fmt.Errorf("unsupported type %s", e.Type)
	}
	return nil
}

func readString(r io.ByteReader) (string, error) {
	var bs []byte
	for b, err := r.ReadByte(); b != 0; b, err = r.ReadByte() {
		if err != nil {
			return "", err
		}
		bs = append(bs, b)
	}
	return string(bs), nil
}
