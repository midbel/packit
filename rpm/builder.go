package rpm

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/packit"
	"github.com/midbel/tape"
	"github.com/midbel/tape/cpio"
)

type builder struct {
	when time.Time

	control *packit.Control
	files   []*packit.File
	changes []*packit.Change
}

func (b *builder) PackageName() string {
	if b.control == nil {
		return "packit.rpm"
	}
	return b.control.PackageName() + ".rpm"
}

func (b *builder) Build(w io.Writer) error {
	if err := b.writeLead(w); err != nil {
		return err
	}
	var data, meta bytes.Buffer
	size, err := b.writeData(&data)
	if err != nil {
		return err
	}

	sh1 := sha1.New()
	if err := b.writeHeader(io.MultiWriter(&meta, sh1)); err != nil {
		return err
	}

	md, sh256 := md5.New(), sha256.New()
	var body bytes.Buffer
	if _, err := io.Copy(io.MultiWriter(&body, md, sh256), io.MultiReader(&meta, &data)); err != nil {
		return err
	}
	var sig bytes.Buffer
	if err := b.writeSums(io.MultiWriter(w, &sig), size, body.Len(), md, sh1, sh256); err != nil {
		return err
	}

	_, err = io.Copy(w, &body)
	return err
}

func (b *builder) writeSums(w io.Writer, data, all int, md, h1, h256 hash.Hash) error {
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

func (b *builder) writeHeader(w io.Writer) error {
	fields := b.controlToFields()
	fields = append(fields, b.filesToFields()...)

	return writeFields(w, fields, rpmTagImmutableIndex, false)
}

func (b *builder) writeData(w io.Writer) (int, error) {
	var data bytes.Buffer
	wc := cpio.NewWriter(&data)

	digest := md5.New()
	for _, i := range b.files {
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
			ModTime:  b.when,
		}
		if err := wc.WriteHeader(&h); err != nil {
			return 0, err
		}
		if i.Size, err = io.Copy(io.MultiWriter(wc, digest), r); err != nil {
			return 0, err
		}
		i.Sum = hex.EncodeToString(digest.Sum(nil))

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

func (b *builder) writeLead(w io.Writer) error {
	body := make([]byte, rpmLeadLen)
	copy(body, rpmMagic)
	binary.BigEndian.PutUint16(body[4:], uint16(rpmMajor)<<8|uint16(rpmMinor))
	binary.BigEndian.PutUint16(body[6:], rpmBinary)
	binary.BigEndian.PutUint16(body[8:], 0)
	if n := []byte(b.control.PackageName()); len(n) <= 65 {
		copy(body[10:], n)
	} else {
		copy(body[10:], n[:65])
	}
	binary.BigEndian.PutUint16(body[76:], 1)
	binary.BigEndian.PutUint16(body[78:], rpmSigType)

	_, err := w.Write(body)
	return err
}

func (b *builder) controlToFields() []rpmField {
	var fs []rpmField
	fs = append(fs, varchar{tag: rpmTagPackage, Value: b.control.Package})
	fs = append(fs, varchar{tag: rpmTagVersion, Value: b.control.Version})
	fs = append(fs, varchar{tag: rpmTagRelease, Value: b.control.Release})
	fs = append(fs, varchar{tag: rpmTagSummary, kind: fieldI18NString, Value: b.control.Summary})
	fs = append(fs, varchar{tag: rpmTagDesc, kind: fieldI18NString, Value: b.control.Desc})
	fs = append(fs, varchar{tag: rpmTagGroup, kind: fieldI18NString, Value: b.control.Section})
	fs = append(fs, varchar{tag: rpmTagOS, Value: b.control.Os})
	fs = append(fs, number{tag: rpmTagBuildTime, kind: fieldInt32, Value: b.when.Unix()})
	fs = append(fs, varchar{tag: rpmTagBuildHost, Value: packit.Hostname()})
	fs = append(fs, varchar{tag: rpmTagDistrib, Value: b.control.Vendor})
	fs = append(fs, varchar{tag: rpmTagVendor, Value: b.control.Vendor})
	fs = append(fs, varchar{tag: rpmTagPackager, Value: "packit"})
	fs = append(fs, varchar{tag: rpmTagLicense, Value: b.control.License})
	fs = append(fs, varchar{tag: rpmTagURL, Value: b.control.Home})
	fs = append(fs, varchar{tag: rpmTagOS, Value: b.control.Os})
	fs = append(fs, varchar{tag: rpmTagArch, Value: Arch(b.control.Arch)})
	fs = append(fs, varchar{tag: rpmTagPayload, Value: rpmPayloadFormat})
	fs = append(fs, varchar{tag: rpmTagCompressor, Value: rpmPayloadCompressor})
	fs = append(fs, varchar{tag: rpmTagPayloadFlags, Value: rpmPayloadFlags})

	if n := len(b.changes); n > 0 {
		ts, cs, ls := make([]int64, n), make([]string, n), make([]string, n)
		m := b.control.Maintainer
		sort.Slice(b.changes, func(i, j int) bool { return b.changes[i].When.After(b.changes[j].When) })
		for i := range b.changes {
			ts[i] = b.changes[i].When.Unix()
			if b.changes[i].Maintainer == nil {
				cs[i] = m.String()
			} else {
				cs[i] = b.changes[i].Maintainer.String()
			}
			ls[i] = strings.Join(b.changes[i].Changes, "\n")
		}
		fs = append(fs, numarray{tag: rpmTagChangeTime, kind: fieldInt32, Value: ts})
		fs = append(fs, strarray{tag: rpmTagChangeName, Values: cs})
		fs = append(fs, strarray{tag: rpmTagChangeText, Values: ls})
	}
	return fs
}

func (b *builder) filesToFields() []rpmField {
	var fs []rpmField

	z := len(b.files)
	dirs, bases := make([]string, z), make([]string, z)
	users, groups := make([]string, z), make([]string, z)
	sizes, digests := make([]string, z), make([]string, z)
	for i := range b.files {
		d, n := filepath.Split(b.files[i].String())
		if !strings.HasPrefix(d, "/") {
			d = "/" + d
		}
		dirs[i], bases[i] = d, n
		users[i], groups[i] = packit.DefaultUser, packit.DefaultGroup
		sizes[i], digests[i] = strconv.FormatInt(b.files[i].Size, 10), b.files[i].Sum

		b.control.Size += b.files[i].Size
	}

	fs = append(fs, number{tag: rpmTagSize, kind: fieldInt32, Value: b.control.Size})
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
