package rpm

import (
	// "crypto/md5"
	// "crypto/sha1"
	// "crypto/sha256"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"
)

func Debug(file string, w io.Writer) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	ws := tabwriter.NewWriter(w, 12, 2, 2, ' ', 0)
	f := dumpEntry(ws)

	if _, err := readLead(r); err != nil {
		return err
	}
	if err := debugEntries(r, true, f); err != nil {
		return err
	}
	ws.Flush()
	fmt.Fprintln(w)
	if err := debugEntries(r, false, f); err != nil {
		return err
	}
	ws.Flush()
	return nil
}

func dumpEntry(w io.Writer) func(e rpmEntry, r io.Reader) error {
	return func(e rpmEntry, r io.Reader) error {
		v, err := e.Decode(r)
		if err != nil {
			return err
		}
		if e.Type == fieldBinary {
			v = hex.EncodeToString(v.([]byte))
		}
		fmt.Fprintf(w, "%d\t%s\t%d\t%v\n", e.Tag, e.Type.String(), e.Len, v)
		return nil
	}
}

func debugEntries(r io.Reader, padding bool, fn func(e rpmEntry, r io.Reader) error) error {
	e := struct {
		Magic uint32
		Spare uint32
		Count int32
		Len   int32
	}{}
	if err := binary.Read(r, binary.BigEndian, &e); err != nil {
		return err
	}
	magic := binary.BigEndian.Uint32(rpmHeader) >> 8
	if e.Magic>>8 != magic {
		return fmt.Errorf("invalid RPM header: %06x", e.Magic)
	}
	if v := e.Magic & 0xFF; byte(v) != rpmHeader[3] {
		return fmt.Errorf("unsupported RPM header version: %d", v)
	}
	size := e.Len
	if m := (e.Len + rpmEntryLen + (e.Count * rpmEntryLen)) % 8; padding && m > 0 {
		size += 8 - m
	}
	es := make([]rpmEntry, int(e.Count))
	for i := 0; i < len(es); i++ {
		if err := binary.Read(r, binary.BigEndian, &es[i]); err != nil {
			return err
		}
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
		if j := i + 1; j < len(es) {
			n = int(es[j].Offset - es[i].Offset)
		}
		if err := fn(e, io.LimitReader(stor, int64(n))); err != nil {
			return err
		}
	}
	return nil
}
