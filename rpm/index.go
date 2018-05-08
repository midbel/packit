package rpm

import (
	"bytes"
	"encoding/binary"
	"io"
)

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

type numarray struct {
	tag   int32
	kind  int32
	Value []int64
}

func (n numarray) Skip() bool { return len(n.Value) == 0 }
func (n numarray) Tag() int32 { return n.tag }
func (n numarray) Type() int32 { return n.kind }
func (n numarray) Len() int32 { return int32(len(n.Value)) }
func (n numarray) Bytes() []byte {
	var w bytes.Buffer
	for _, v := range n.Value {
		writeNumber(&w, n.kind, v)
	}
	return w.Bytes()
}

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
	writeNumber(&w, n.kind, n.Value)
	return w.Bytes()
}

func writeNumber(w io.Writer, t int32, n int64) {
	switch EntryType(t) {
	case Int8:
		binary.Write(w, binary.BigEndian, int8(n))
	case Int16:
		binary.Write(w, binary.BigEndian, int16(n))
	case Int32:
		binary.Write(w, binary.BigEndian, int32(n))
	case Int64:
		binary.Write(w, binary.BigEndian, int64(n))
	}
}

type varchar struct {
	tag   int32
	kind  int32
	Value string
}

func (v varchar) Skip() bool { return len(v.Value) == 0 }
func (v varchar) Tag() int32 { return v.tag }
func (v varchar) Type() int32 {
	if v.kind == 0 {
		return int32(String)
	}
	return v.kind
}
func (v varchar) Len() int32 { return 1 }
func (v varchar) Bytes() []byte {
	return append([]byte(v.Value), 0)
}

type strarray struct {
	tag   int32
	Value []string
}

func (a strarray) Skip() bool  { return len(a.Value) == 0 }
func (a strarray) Tag() int32  { return a.tag }
func (a strarray) Type() int32 { return int32(StringArray) }
func (a strarray) Len() int32  { return int32(len(a.Value)) }
func (a strarray) Bytes() []byte {
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
