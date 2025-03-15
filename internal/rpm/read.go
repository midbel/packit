package rpm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

func readLead(r io.Reader) error {
	lead := make([]byte, rpmLeadLen)
	if _, err := io.ReadFull(r, lead); err != nil {
		return err
	}
	if !bytes.HasPrefix(lead, rpmMagic) {
		return fmt.Errorf("not a rpm file - invalid magic (%x)", lead[:len(rpmMagic)])
	}
	return nil
}

func readHeader(r io.Reader, index, store io.Writer, padded bool) error {
	var (
		buf   = make([]byte, len(rpmHeader))
		count int32
		size  int32
	)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	if !bytes.Equal(buf, rpmHeader) {
		return fmt.Errorf("header: not a valid rpm header (%x)", buf)
	}
	binary.Read(r, binary.BigEndian, &count)
	binary.Read(r, binary.BigEndian, &size)

	if _, err := io.CopyN(index, r, int64(rpmEntryLen*count)); err != nil {
		return err
	}
	if _, err := io.CopyN(store, r, int64(size)); err != nil {
		return err
	}

	if padded {
		if mod := size % 8; mod != 0 {
			zs := make([]byte, 8-mod)
			io.ReadFull(r, zs)
		}
	}
	return nil
}
