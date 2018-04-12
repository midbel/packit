package rpm

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
  "sort"
)

const MagicRPM = 0xedabeedb
const MagicHDR = 0x008eade8

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
  h.store = new(bytes.Buffer)
  if _, err := io.CopyN(h.store, r, int64(h.Length)); err != nil {
    return nil, err
  }
  if mod := h.Length%8; mod != 0 {
    r.Discard(8-int(mod))
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
