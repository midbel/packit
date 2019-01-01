package rpm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/midbel/packit"
	"github.com/midbel/tape/cpio"
)

type rpm struct {
	name string

	data *bytes.Reader
}

func (r *rpm) PackageName() string {
	return r.name
}

func (r *rpm) Valid() error {
	return nil
}

func (r *rpm) About() packit.Control {
	var c packit.Control
	return c
}

func (r *rpm) Filenames() ([]string, error) {
	if _, err := r.data.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	rs := cpio.NewReader(r.data)
	var vs []string
	for {
		h, err := rs.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		vs = append(vs, h.Filename)
		if _, err := io.CopyN(ioutil.Discard, rs, h.Length); err != nil {
			return nil, err
		}
	}
	return vs, nil
}

func readLead(r io.Reader, p *pkg) error {
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
	p.name = string(bytes.Trim(c.Name[:], "\x00"))
	return nil
}

func readSignature(r io.Reader, p *pkg) error {
	return nil
}

func readHeader(r io.Reader, p *pkg) error {
	return nil
}

func readData(r io.Reader, p *pkg) error {
	return nil
}
