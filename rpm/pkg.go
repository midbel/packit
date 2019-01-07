package rpm

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/midbel/packit"
	"github.com/midbel/tape/cpio"
)

type pkg struct {
	name string

	control *packit.Control
	history packit.History

	data *bytes.Reader
}

type signature struct {
	Payload int64
	Size    int64
	Sha1    string
	Sha256  string
	MD5     string
}

func (p *pkg) PackageType() string {
	return "rpm"
}

func (p *pkg) PackageName() string {
	return p.name
}

func (p *pkg) Valid() error {
	return nil
}

func (p *pkg) About() packit.Control {
	return *p.control
}

func (p *pkg) History() packit.History {
	return p.history
}

func (p *pkg) Resources() ([]packit.Resource, error) {
	if p.data == nil {
		return nil, packit.ErrUnsupportedPayloadFormat
	}
	if _, err := p.data.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	r := cpio.NewReader(p.data)
	var rs []packit.Resource
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		e := packit.Resource{
			Name:    h.Filename,
			Size:    h.Length,
			ModTime: h.ModTime,
			Perm:    h.Mode,
		}
		rs = append(rs, e)
		if _, err := io.CopyN(ioutil.Discard, r, h.Length); err != nil {
			return nil, err
		}
	}
	return rs, nil
}

func (p *pkg) Filenames() ([]string, error) {
	rs, err := p.Resources()
	if err != nil {
		return nil, err
	}
	vs := make([]string, len(rs))
	for i := 0; i < len(rs); i++ {
		vs[i] = rs[i].Name
	}
	return vs, nil
}

func (p *pkg) Extract(datadir string, preserve bool) error {
	if p.data == nil {
		return packit.ErrUnsupportedPayloadFormat
	}
	if err := os.MkdirAll(datadir, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if _, err := p.data.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r := cpio.NewReader(p.data)
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		dir, _ := filepath.Split(h.Filename)
		if err := os.MkdirAll(filepath.Join(datadir, dir), 0755); err != nil {
			return err
		}
		w, err := os.Create(filepath.Join(datadir, h.Filename))
		if err != nil {
			return err
		}
		if _, err := io.CopyN(w, r, h.Length); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
	}
	return nil
}

func readMeta(r io.Reader) (*packit.Control, packit.History, error) {
	var (
		c   packit.Control
		pay string // payload format, should be cpio
		com string // payload compressor, should be gz, xz,...
	)

	var (
		ctimes []int64
		cnames []string
		clogs  []string
	)
	err := readHeader(r, false, func(tag int32, v interface{}) error {
		switch tag {
		case rpmTagChangeTime:
			ctimes = v.([]int64)
		case rpmTagChangeName:
			cnames = v.([]string)
		case rpmTagChangeText:
			clogs = v.([]string)
		case rpmTagSize:
			if xs, ok := v.([]int64); ok && len(xs) == 1 {
				c.Size = xs[0]
			}
		case rpmTagBuildTime:
			if xs, ok := v.([]int64); ok && len(xs) == 1 {
				c.Date = time.Unix(xs[0], 0)
			}
		case rpmTagPackage:
			c.Package = v.(string)
		case rpmTagVersion:
			c.Version = v.(string)
		case rpmTagRelease:
			c.Release = v.(string)
		case rpmTagSummary:
			c.Summary = v.(string)
		case rpmTagDesc:
			c.Desc = v.(string)
		case rpmTagPackager:
			if m, err := packit.ParseMaintainer(v.(string)); err == nil {
				c.Maintainer = m
			}
		case rpmTagVendor:
			c.Vendor = v.(string)
		case rpmTagLicense:
			c.License = v.(string)
		case rpmTagGroup:
			c.Section = v.(string)
		case rpmTagURL:
			c.Home = v.(string)
		case rpmTagArch:
			switch x := v.(string); x {
			case "x86_64":
				c.Arch = packit.Arch64
			case "i386":
				c.Arch = packit.Arch32
			}
		case rpmTagPayload:
			pay = v.(string)
		case rpmTagCompressor:
			com = v.(string)
		case rpmTagPayloadFlags:
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	var cs []packit.Change
	for i := 0; i < len(clogs); i++ {
		c := packit.Change{
			When: time.Unix(ctimes[i], 0),
			Body: clogs[i],
		}
		if m, v, err := packit.ParseMaintainerVersion(cnames[i]); err == nil {
			c.Version, c.Maintainer = v, m
		}
		cs = append(cs, c)
	}
	if pay != "" && com != "" {
		c.Format = fmt.Sprintf("%s.%s", pay, com)
	}
	return &c, packit.History(cs), nil
}

func readSignature(r io.Reader) (*signature, error) {
	s := signature{
		Payload: -1,
		Size:    -1,
	}
	return &s, readHeader(r, true, func(tag int32, v interface{}) error {
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
			if xs, ok := v.([]int64); ok && len(xs) == 1 {
				s.Size = xs[0]
			}
		case rpmSigPayload:
			if xs, ok := v.([]int64); ok && len(xs) == 1 {
				s.Payload = xs[0]
			}
		}
		return nil
	})
}

func readData(r io.Reader, format string) (*bytes.Reader, error) {
	var (
		z   io.Reader
		err error
	)
	switch format {
	case "cpio.gz", "cpio.gzip", "":
		z, err = gzip.NewReader(r)
	case "cpio.xz":
		return nil, packit.ErrUnsupportedPayloadFormat
	default:
		return nil, packit.ErrMalformedPackage
	}
	if err != nil {
		return nil, err
	}
	xs, err := ioutil.ReadAll(z)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(xs), nil
}

func readLead(r io.Reader) (string, error) {
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
		return "", err
	}
	if c.Magic != binary.BigEndian.Uint32(rpmMagic) {
		return "", fmt.Errorf("invalid RPM magic: %08x", c.Magic)
	}
	if c.Major != rpmMajor {
		return "", fmt.Errorf("unsupported RPM version: %d.%d", c.Major, c.Minor)
	}
	if c.Signature != rpmSigType {
		return "", fmt.Errorf("invalid RPM signature type: %d", c.Signature)
	}
	return string(bytes.Trim(c.Name[:], "\x00")), nil
}

func readHeader(r io.Reader, padding bool, fn func(tag int32, v interface{}) error) error {
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
		v, err := e.Decode(io.LimitReader(stor, int64(n)))
		if err != nil {
			return err
		}
		if v == nil {
			continue
		}
		if err := fn(e.Tag, v); err != nil {
			return err
		}
	}
	return nil
}

type rpmEntry struct {
	Tag    int32
	Type   fieldType
	Offset int32
	Len    int32
}

func (e rpmEntry) Decode(r io.Reader) (interface{}, error) {
	var (
		v   interface{}
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
		vs := make([]int64, e.Len)
		for i := 0; i < len(vs); i++ {
			var j int32
			if err = binary.Read(r, binary.BigEndian, &j); err != nil {
				break
			}
			vs[i] = int64(j)
		}
		v = vs
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
		return copy(xs, bs) + 1, xs, nil
	}
}
