package rpm

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/midbel/packit"
	"github.com/midbel/tape"
	"github.com/midbel/tape/cpio"
	"github.com/midbel/textwrap"
)

const (
	rpmArchAll = "noarch"
	rpmArch64  = "x86_64"
	rpmArch32  = "i386"

	rpmLeadLen       = 96
	rpmEntryLen      = 16
	rpmMajorVersion  = 3
	rpmMinorVersion  = 0
	rpmBinaryPackage = 0
	rpmSigType       = 5
	rpmLinuxOS       = 1
)

var (
	rpmMagicRpm    = []byte{0xed, 0xab, 0xee, 0xdb}
	rpmMagicHeader = []byte{0x8e, 0xad, 0xe8, 0x01}
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
	rpmSigSha1    = 269
	rpmSigSha256  = 273
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
	rpmTagProvide          = 1047
	rpmTagProvideVersion   = 1113
	rpmTagProvideFlag      = 1112
	rpmTagRequire          = 1049
	rpmTagRequireVersion   = 1050
	rpmTagRequireFlag      = 1048
	rpmTagConflict         = 1054
	rpmTagConflictVersion  = 1055
	rpmTagConflictFlag     = 1053
	rpmTagObsolete         = 1090
	rpmTagObsoleteVersion  = 1115
	rpmTagObsoleteFlag     = 1114
	rpmTagRecommand        = 5046
	rpmTagRecommandVersion = 5047
	rpmTagRecommandFlag    = 5048
	rpmTagSuggest          = 5049
	rpmTagSuggestVersion   = 5050
	rpmTagSuggestFlag      = 5051
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
	rpmTagPrein       = 1023
	rpmTagPreinFlags  = 5020
	rpmTagPreinProg   = 1085
	rpmTagPostin      = 1024
	rpmTagPostinFlags = 5021
	rpmTagPostinProg  = 1086
	rpmTagPreun       = 1025
	rpmTagPreunFlags  = 5024
	rpmTagPreunProg   = 1087
	rpmTagPostun      = 1026
	rpmTagPostunFlags = 5023
	rpmTagPostunProg  = 1088
)

const (
	rpmTagFilenames = 5000
	rpmTagBugURL    = 5012
	rpmTagEncoding  = 5068
)

func Build(dir string, meta packit.Metadata) error {
	w, err := os.Create(filepath.Join(dir, getPackageName(meta)))
	if err != nil {
		return err
	}
	defer w.Close()
	return build(w, meta)
}

func build(w io.Writer, meta packit.Metadata) error {
	if err := writeLead(w, meta); err != nil {
		return err
	}
	var data bytes.Buffer
	if err := writeData(&data, meta); err != nil {
		return err
	}
	var (
		header bytes.Buffer
		sh1    = sha1.New()
		sh2    = sha256.New()
	)
	if err := writeHeader(io.MultiWriter(sh1, sh2, &header), meta); err != nil {
		return err
	}
	var (
		body bytes.Buffer
		md   = md5.New()
	)
	_, err := io.Copy(io.MultiWriter(md, &body), io.MultiReader(&header, &data))
	if err != nil {
		return err
	}
	err = writeSignature(w, data.Len(), body.Len(), md, sh1, sh2)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, &body)
	return err
}

func writeSignature(w io.Writer, archive, size int, m, h1, h2 hash.Hash) error {
	var (
		sum = m.Sum(nil)
		fs  = []field{
			getNumber32(rpmSigLength, int64(size)),
			getNumber32(rpmSigPayload, int64(archive)),
			getString(rpmSigSha1, fmt.Sprintf("%x", h1.Sum(nil))),
			getString(rpmSigSha256, fmt.Sprintf("%x", h2.Sum(nil))),
			getBinary(rpmSigMD5, sum[:]),
		}
	)
	return writeFields(w, fs, rpmTagSignatureIndex, true)
}

func writeHeader(w io.Writer, meta packit.Metadata) error {
	fields := getFields(meta)
	return writeFields(w, fields, rpmTagImmutableIndex, false)
}

func writeFields(w io.Writer, fields []field, tag int, pad bool) error {
	write := func(stor, hdr *bytes.Buffer, f field) {
		if limit := f.Limit(); limit > 0 {
			if mod := stor.Len() % limit; mod != 0 {
				for i := 0; i < limit-mod; i++ {
					stor.WriteByte(0)
				}
			}
		}
		binary.Write(hdr, binary.BigEndian, f.Tag)
		binary.Write(hdr, binary.BigEndian, f.Kind)
		binary.Write(hdr, binary.BigEndian, int32(stor.Len()))
		binary.Write(hdr, binary.BigEndian, f.Len())
		stor.Write(f.Bytes())
	}
	var (
		hdr   bytes.Buffer
		idx   bytes.Buffer
		stor  bytes.Buffer
		count int32
	)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Tag < fields[j].Tag
	})
	for _, f := range fields {
		if f.Skip() {
			continue
		}
		write(&stor, &hdr, f)
		count++
	}
	if tag > 0 {
		count++
		binary.Write(&idx, binary.BigEndian, uint32(tag))
		binary.Write(&idx, binary.BigEndian, uint32(kindBinary))
		binary.Write(&idx, binary.BigEndian, int32(stor.Len()))
		binary.Write(&idx, binary.BigEndian, int32(rpmEntryLen))

		binary.Write(&stor, binary.BigEndian, uint32(tag))
		binary.Write(&stor, binary.BigEndian, uint32(kindBinary))
		binary.Write(&stor, binary.BigEndian, int32(-hdr.Len()-rpmEntryLen))
		binary.Write(&stor, binary.BigEndian, int32(rpmEntryLen))
	}

	w.Write(rpmMagicHeader)
	binary.Write(w, binary.BigEndian, uint32(0))
	binary.Write(w, binary.BigEndian, count)
	binary.Write(w, binary.BigEndian, int32(stor.Len()))

	n, err := io.Copy(w, io.MultiReader(&idx, &hdr, &stor))
	if m := n % 8; m != 0 && pad {
		w.Write(make([]byte, 8-m))
	}
	return err
}

func writeData(w io.Writer, meta packit.Metadata) error {
	var (
		buf, _ = gzip.NewWriterLevel(w, gzip.BestCompression)
		tmp    = cpio.NewWriter(buf)
	)
	defer func() {
		tmp.Close()
		buf.Close()
	}()
	for i := range meta.Resources {
		if err := appendResource(tmp, meta.Resources[i]); err != nil {
			return err
		}
	}
	return nil
}

func appendResource(cw *cpio.Writer, res packit.Resource) error {
	r, err := os.Open(res.File)
	if err != nil {
		return err
	}
	defer r.Close()

	var (
		buf bytes.Buffer
		w   io.Writer = &buf
	)
	if res.Compress {
		w, _ = gzip.NewWriterLevel(w, gzip.BestCompression)
	}
	_, err = io.Copy(w, r)
	if c, ok := w.(io.Closer); ok {
		c.Close()
	}
	h := getHeader(res.Path(), res.Size, res.ModTime)
	if err := cw.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(cw, &buf)
	return err
}

func getHeader(file string, size int64, when time.Time) tape.Header {
	return tape.Header{
		Filename: file,
		Uid:      0,
		Gid:      0,
		Mode:     0644,
		Length:   int64(size),
		ModTime:  when,
	}
}

func writeLead(w io.Writer, meta packit.Metadata) error {
	var (
		lead    = make([]byte, rpmLeadLen)
		version = uint16(rpmMajorVersion)<<8 | uint16(rpmMinorVersion)
		name    = []byte(meta.PackageName())
	)
	copy(lead, rpmMagicRpm)
	binary.BigEndian.PutUint16(lead[4:], version)
	binary.BigEndian.PutUint16(lead[6:], uint16(rpmBinaryPackage))
	binary.BigEndian.PutUint16(lead[8:], uint16(0)) // architecture
	if len(name) >= 66 {
		name = name[:65]
	}
	copy(lead[10:], name)
	binary.BigEndian.PutUint16(lead[76:], uint16(rpmLinuxOS))
	binary.BigEndian.PutUint16(lead[78:], uint16(rpmSigType))

	_, err := w.Write(lead)
	return err
}

func getFields(meta packit.Metadata) []field {
	fs := getBaseFields(meta)
	fs = append(fs, getFileFields(meta.Resources)...)
	fs = append(fs, getDependencyFields(meta)...)
	fs = append(fs, getScriptFields(meta)...)
	fs = append(fs, getChangeFields(meta.Changes, meta.Maintainer)...)
	return fs
}

func getBaseFields(meta packit.Metadata) []field {
	return []field{
		getNumber32(rpmTagSize, meta.Size),
		getString(rpmTagPackage, meta.Package),
		getString(rpmTagVersion, meta.Version),
		getString(rpmTagRelease, meta.Release),
		getStringI18N(rpmTagSummary, meta.Summary),
		getStringI18N(rpmTagDesc, meta.Desc),
		getStringI18N(rpmTagGroup, meta.Section),
		getString(rpmTagOS, meta.OS),
		getString(rpmTagBuildHost, packit.Hostname()),
		getNumber32(rpmTagBuildTime, meta.Date.Unix()),
		getString(rpmTagVendor, meta.Vendor),
		getString(rpmTagPackager, meta.Maintainer.String()),
		getString(rpmTagLicense, meta.License),
		getString(rpmTagURL, meta.Home),
		getString(rpmTagArch, getPackageArch(meta.Arch)),
		getString(rpmTagPayload, rpmPayloadFormat),
		getString(rpmTagCompressor, rpmPayloadCompressor),
		getString(rpmTagPayloadFlags, rpmPayloadFlags),
	}
}

func getDependencyFields(meta packit.Metadata) []field {
	return nil
}

func getScriptFields(meta packit.Metadata) []field {
	return []field{
		getString(rpmTagPrein, meta.PreInst.Code),
		getString(rpmTagPostin, meta.PostInst.Code),
		getString(rpmTagPreun, meta.PreRem.Code),
		getString(rpmTagPostun, meta.PostRem.Code),
	}
	return nil
}

func getFileFields(resources []packit.Resource) []field {
	var (
		dirs    []string
		bases   []string
		files   []string
		users   []string
		groups  []string
		sizes   []int64
		digests []string
		times   []int64
		seen    = make(map[string]struct{})
	)
	for _, r := range resources {
		files = append(files, r.Path())
		times = append(times, r.ModTime.Unix())
		sizes = append(sizes, r.Size)
		digests = append(digests, r.Digest)
		users = append(users, packit.Root)
		groups = append(groups, packit.Root)
		bases = append(bases, filepath.Base(r.Path()))
		dirs = append(dirs, getListDirectories(r.Path(), seen)...)
	}
	return []field{
		getArrayNumber32(rpmTagFileTimes, times),
		getArrayNumber32(rpmTagFileSizes, sizes),
		getArrayString(rpmTagFileDigests, digests),
		getArrayString(rpmTagDirnames, dirs),
		getArrayString(rpmTagBasenames, bases),
		getArrayString(rpmTagFilenames, files),
		getArrayString(rpmTagOwners, users),
		getArrayString(rpmTagGroups, groups),
	}
}

func getChangeFields(changes []packit.Change, maintainer packit.Maintainer) []field {
	var fs []field
	if len(changes) == 0 {
		return fs
	}
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].When.Before(changes[j].When)
	})
	var (
		ms []string
		ds []string
		ts []int64
	)
	for i, c := range changes {
		ts = append(ts, c.When.Unix())
		if c.Maintainer.IsZero() {
			ms = append(ms, maintainer.String())
		} else {
			ms = append(ms, c.Maintainer.String())
		}
		if c.Version != "" {
			ms[i] += "-" + c.Version
		}
		ds = append(ds, textwrap.Wrap(c.Desc))
	}
	fs = append(fs, getArrayNumber32(rpmTagChangeTime, ts))
	fs = append(fs, getArrayString(rpmTagChangeName, ms))
	fs = append(fs, getArrayString(rpmTagChangeText, ds))

	return fs
}

func getListDirectories(file string, done map[string]struct{}) []string {
	var (
		dirs []string
		tmp  string
		dir  = filepath.Dir(file)
	)
	for _, d := range strings.Split(dir, "/") {
		tmp = filepath.Join(tmp, d)
		if _, ok := done[tmp]; ok {
			continue
		}
		done[tmp] = struct{}{}
		dirs = append(dirs, tmp)
	}
	return dirs
}

const namepat = "%s-%s.%s.%s"

func getPackageName(meta packit.Metadata) string {
	arch := getPackageArch(meta.Arch)
	return fmt.Sprintf(namepat, meta.Package, meta.Version, arch, packit.RPM)
}

func getPackageArch(arch int) string {
	switch arch {
	case packit.Arch64:
		return rpmArch64
	case packit.Arch32:
		return rpmArch32
	default:
		return rpmArchAll
	}
}

type kind uint32

const (
	kindNull kind = iota
	kindChar
	kindInt8
	kindInt16
	kindInt32
	kindInt64
	kindString
	kindBinary
	kindStringArray
	kindI18nString
)

type field struct {
	Kind  kind
	Tag   int32
	bytes [][]byte
}

func getBinary(tag int32, str []byte) field {
	var b [][]byte
	return field{
		Tag:   tag,
		Kind:  kindBinary,
		bytes: append(b, str),
	}
}

func getArrayNumber32(tag int32, list []int64) field {
	var b [][]byte
	for i := range list {
		b = append(b, itob(list[i], 4))
	}
	return field{
		Tag:   tag,
		Kind:  kindInt32,
		bytes: b,
	}
}

func getNumber32(tag int32, num int64) field {
	var b [][]byte
	return field{
		Tag:   tag,
		Kind:  kindInt32,
		bytes: append(b, itob(num, 4)),
	}
}

func itob(n int64, z int) []byte {
	var (
		b = make([]byte, z)
		x int
	)
  for i := z-1; i >= 0; i-- {
    b[i] = byte(n>>x)
    x += 8
  }
	return b
}

func getArrayString(tag int32, list []string) field {
	var b [][]byte
	for i := range list {
		b = append(b, []byte(list[i]+"\x00"))
	}
	return field{
		Tag:   tag,
		Kind:  kindStringArray,
		bytes: b,
	}
}

func getString(tag int32, str string) field {
	var b [][]byte
	return field{
		Tag:   tag,
		Kind:  kindString,
		bytes: append(b, []byte(str+"\x00")),
	}
}

func getStringI18N(tag int32, str string) field {
	var b [][]byte
	return field{
		Tag:   tag,
		Kind:  kindI18nString,
		bytes: append(b, []byte(str+"\x00")),
	}
}

func getIndex(tag int) field {
	return field{}
}

func (f field) Bytes() []byte {
	var b []byte
	for i := range f.bytes {
		b = append(b, f.bytes[i]...)
	}
	return b
}

func (f field) Len() int32 {
	if f.Kind == kindBinary {
		b := f.Bytes()
		return int32(len(b))
	}
	return int32(len(f.bytes))
}

func (f field) Skip() bool {
	return len(f.bytes) == 0
}

func (f field) Limit() int {
	var limit int
	switch f.Kind {
	case kindInt8:
		limit = 1
	case kindInt16:
		limit = 2
	case kindInt32:
		limit = 4
	case kindInt64:
		limit = 8
	}
	return limit
}
