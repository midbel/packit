package rpm

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/midbel/packit/internal/packfile"
	"github.com/midbel/tape"
	"github.com/midbel/tape/cpio"
)

type RpmBuilder struct {
	writer    *bufio.Writer
	buildTime time.Time
	buildHost string
}

func Build(w io.Writer) (*RpmBuilder, error) {
	b := RpmBuilder{
		writer:    bufio.NewWriter(w),
		buildTime: time.Now(),
		buildHost: "localhost",
	}
	if host, err := os.Hostname(); err == nil {
		b.buildHost = host
	}
	return &b, nil
}

func (b *RpmBuilder) Build(p *packfile.Package) error {
	if err := b.setup(p); err != nil {
		return err
	}
	if err := b.build(p); err != nil {
		return err
	}
	return b.teardown(p)
}

func (b *RpmBuilder) Close() error {
	return b.writer.Flush()
}

func (b *RpmBuilder) setup(pkg *packfile.Package) error {
	if pkg.Setup == "" {
		return nil
	}
	scan := bufio.NewScanner(strings.NewReader(pkg.Setup))
	for scan.Scan() {
		cmd := exec.Command("sh", "-c", scan.Text())
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (b *RpmBuilder) teardown(pkg *packfile.Package) error {
	if pkg.Teardown == "" {
		return nil
	}
	scan := bufio.NewScanner(strings.NewReader(pkg.Teardown))
	for scan.Scan() {
		cmd := exec.Command("sh", "-c", scan.Text())
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (b *RpmBuilder) build(p *packfile.Package) error {
	data, err := writeFiles(p)
	if err != nil {
		return err
	}
	defer func() {
		data.Close()
		os.Remove(data.Name())
	}()

	if err := b.writeLead(p); err != nil {
		return err
	}
	var (
		sh1 = sha1.New()
		hdr bytes.Buffer
	)
	if err := b.prepareHeader(p, io.MultiWriter(&hdr, sh1)); err != nil {
		return err
	}
	var (
		md = md5.New()
		sh = sha256.New()
		by = hdr.Bytes()
	)
	data.Seek(0, os.SEEK_SET)
	totalSize, err := io.Copy(io.MultiWriter(md, sh), io.MultiReader(bytes.NewReader(by), data))
	if err != nil {
		return err
	}

	stat, err := data.Stat()
	if err != nil {
		return err
	}
	if err := b.writeSignatures(stat.Size(), totalSize, md, sh1, sh); err != nil {
		return err
	}
	data.Seek(0, os.SEEK_SET)
	_, err = io.Copy(b.writer, io.MultiReader(bytes.NewReader(by), data))
	return err
}

func (b *RpmBuilder) writeSignatures(data, total int64, md, sh1, sh2 hash.Hash) error {
	var (
		index bytes.Buffer
		store bytes.Buffer
	)

	writeHashString(&index, &store, rpmSigSha1, fieldString, sh1)
	writeHashString(&index, &store, rpmSigSha256, fieldString, sh2)
	writeIntEntry(&index, &store, rpmSigLength, fieldInt32, total)
	writeHashBinary(&index, &store, rpmSigMD5, fieldBinary, md)
	writeIntEntry(&index, &store, rpmSigPayload, fieldInt32, data)

	var (
		tmp       bytes.Buffer
		itemCount = index.Len() / rpmEntryLen
	)

	tmp.Write(rpmHeader)
	binary.Write(&tmp, binary.BigEndian, int32(itemCount+1))
	binary.Write(&tmp, binary.BigEndian, int32(store.Len()+rpmEntryLen))

	binary.Write(&tmp, binary.BigEndian, int32(rpmTagSignatureIndex))
	binary.Write(&tmp, binary.BigEndian, int32(fieldBinary))
	binary.Write(&tmp, binary.BigEndian, int32(store.Len()))
	binary.Write(&tmp, binary.BigEndian, int32(rpmEntryLen))

	binary.Write(&store, binary.BigEndian, int32(rpmTagSignatureIndex))
	binary.Write(&store, binary.BigEndian, int32(fieldBinary))
	binary.Write(&store, binary.BigEndian, int32(-index.Len()-rpmEntryLen))
	binary.Write(&store, binary.BigEndian, int32(rpmEntryLen))

	n, _ := io.Copy(&tmp, io.MultiReader(&index, &store))
	if mod := n % 8; mod != 0 {
		zeros := make([]byte, 8-mod)
		tmp.Write(zeros)
	}

	_, err := io.Copy(b.writer, &tmp)
	return err
}

func (b *RpmBuilder) prepareHeader(p *packfile.Package, ws io.Writer) error {
	var (
		index bytes.Buffer
		store bytes.Buffer
	)
	writeStringEntry(&index, &store, rpmTagPackage, fieldString, p.Name)
	writeStringEntry(&index, &store, rpmTagVersion, fieldString, p.Version)
	writeStringEntry(&index, &store, rpmTagRelease, fieldString, p.Release)
	writeStringEntry(&index, &store, rpmTagSummary, fieldI18NString, p.Summary)
	writeStringEntry(&index, &store, rpmTagDesc, fieldI18NString, p.Desc)
	writeIntEntry(&index, &store, rpmTagBuildTime, fieldInt32, b.buildTime.Unix())
	writeStringEntry(&index, &store, rpmTagBuildHost, fieldString, b.buildHost)
	writeIntEntry(&index, &store, rpmTagSize, fieldInt32, p.TotalSize())
	writeStringEntry(&index, &store, rpmTagDistrib, fieldString, p.Distrib)
	writeStringEntry(&index, &store, rpmTagVendor, fieldString, p.Vendor)
	writeStringEntry(&index, &store, rpmTagLicense, fieldString, p.License)
	writeStringEntry(&index, &store, rpmTagPackager, fieldString, p.Maintainer.Name)
	writeStringEntry(&index, &store, rpmTagGroup, fieldI18NString, p.Section)
	writeStringEntry(&index, &store, rpmTagURL, fieldString, p.Home)
	writeStringEntry(&index, &store, rpmTagArch, fieldString, p.Arch)

	prepareChanges(p, &index, &store)
	prepareFiles(p, &index, &store)
	prepareScripts(p, &index, &store)
	prepareDependencies(p, &index, &store)

	writeStringEntry(&index, &store, rpmTagPayload, fieldString, rpmPayloadFormat)
	writeStringEntry(&index, &store, rpmTagCompressor, fieldString, rpmPayloadCompressor)
	writeStringEntry(&index, &store, rpmTagPayloadFlags, fieldString, rpmPayloadFlags)

	var (
		tmp       bytes.Buffer
		itemCount = index.Len() / rpmEntryLen
	)

	tmp.Write(rpmHeader)
	binary.Write(&tmp, binary.BigEndian, int32(itemCount+1))
	binary.Write(&tmp, binary.BigEndian, int32(store.Len()+rpmEntryLen))

	binary.Write(&tmp, binary.BigEndian, int32(rpmTagImmutableIndex))
	binary.Write(&tmp, binary.BigEndian, int32(fieldBinary))
	binary.Write(&tmp, binary.BigEndian, int32(store.Len()))
	binary.Write(&tmp, binary.BigEndian, int32(rpmEntryLen))

	binary.Write(&store, binary.BigEndian, int32(rpmTagImmutableIndex))
	binary.Write(&store, binary.BigEndian, int32(fieldBinary))
	binary.Write(&store, binary.BigEndian, int32(-index.Len()-rpmEntryLen))
	binary.Write(&store, binary.BigEndian, int32(rpmEntryLen))

	_, err := io.Copy(ws, io.MultiReader(&tmp, &index, &store))
	return err
}

func (b *RpmBuilder) writeLead(p *packfile.Package) error {
	body := make([]byte, rpmLeadLen)
	copy(body, rpmMagic)

	binary.BigEndian.PutUint16(body[4:], uint16(rpmMajor)<<8|uint16(rpmMinor))
	binary.BigEndian.PutUint16(body[6:], rpmBinary)
	binary.BigEndian.PutUint16(body[8:], 0)
	if n := []byte(p.PackageName()); len(n) <= 65 {
		copy(body[10:], n)
	} else {
		copy(body[10:], n[:65])
	}
	binary.BigEndian.PutUint16(body[76:], 1)
	binary.BigEndian.PutUint16(body[78:], rpmSigType)

	_, err := b.writer.Write(body)
	return err
}

const (
	fileBasePerm = -1 << 15
	dirBasePerm  = 1 << 14
)

func pathToSlash(str string) string {
	return strings.ReplaceAll(str, "\\", "/")
}

func pathToRoot(str string) string {
	if !strings.HasPrefix(str, "/") {
		str = "/" + str
	}
	return str
}

func prepareFiles(p *packfile.Package, index, store *bytes.Buffer) error {
	var (
		dirs    []string
		bases   []string
		flags   []int64
		devs    []int64
		inodes  []int64
		indexes []int64
		perms   []int64
		users   []string
		groups  []string
		sizes   []int64
		times   []int64
		digests []string
		links   []string
		langs   []string
		now     = time.Now()
	)
	for _, f := range p.Files {
		f.Target = pathToSlash(f.Target)
		dir, base := path.Split(f.Target)

		dir = pathToRoot(dir)
		if ok := slices.Contains(dirs, dir); !ok {
			dirs = append(dirs, dir)

			tmp := strings.TrimSuffix(dir, "/")
			tmp = path.Base(tmp)

			parent := path.Dir(strings.TrimSuffix(dir, "/")) + "/"
			if ix := slices.Index(dirs, parent); ix < 0 {
				dirs = append(dirs, parent)
			}

			indexes = append(indexes, int64(slices.Index(dirs, parent)))
			bases = append(bases, tmp)
			perms = append(perms, dirBasePerm+f.Perm)
			sizes = append(sizes, 0)
			times = append(times, now.Unix())
			digests = append(digests, "")
			users = append(users, packfile.DefaultUser)
			groups = append(groups, packfile.DefaultGroup)
			// flags = append(flags, rpmFileDir)
			devs = append(devs, 0)
			flags = append(flags, packfile.FileFlagDir)
			inodes = append(inodes, int64(len(bases))+1)
			links = append(links, "")
			langs = append(langs, "")
		}

		indexes = append(indexes, int64(slices.Index(dirs, dir)))
		bases = append(bases, base)
		perms = append(perms, fileBasePerm+f.Perm)
		sizes = append(sizes, f.Size)
		times = append(times, now.Unix())
		digests = append(digests, f.Hash)
		users = append(users, packfile.DefaultUser)
		groups = append(groups, packfile.DefaultGroup)
		flags = append(flags, f.Flags)
		devs = append(devs, 0)
		inodes = append(inodes, int64(len(bases))+1)
		links = append(links, "")
		langs = append(langs, "")
	}

	writeIntArrayEntry(index, store, rpmTagFileSizes, fieldInt32, sizes)
	writeIntArrayEntry(index, store, rpmTagFileModes, fieldInt16, perms)
	writeIntArrayEntry(index, store, rpmTagFileDevs, fieldInt16, devs)
	writeIntArrayEntry(index, store, rpmTagFileTimes, fieldInt32, times)
	writeStringArrayEntry(index, store, rpmTagFileDigests, fieldStrArray, digests)
	writeStringArrayEntry(index, store, rpmTagOwners, fieldStrArray, users)
	writeStringArrayEntry(index, store, rpmTagGroups, fieldStrArray, groups)
	writeIntArrayEntry(index, store, rpmTagFileInodes, fieldInt32, inodes)
	writeIntArrayEntry(index, store, rpmTagDirIndexes, fieldInt32, indexes)
	writeStringArrayEntry(index, store, rpmTagBasenames, fieldStrArray, bases)
	writeStringArrayEntry(index, store, rpmTagDirnames, fieldStrArray, dirs)
	writeIntArrayEntry(index, store, rpmTagFileFlags, fieldInt32, flags)
	writeStringArrayEntry(index, store, rpmTagFileLinks, fieldStrArray, links)
	writeStringArrayEntry(index, store, rpmTagFileLangs, fieldStrArray, langs)
	return nil
}

func prepareDependencies(p *packfile.Package, index, store *bytes.Buffer) error {
	writeDeps := func(it []packfile.Dependency, nameTag, versionTag, flagTag int) {
		if len(it) == 0 {
			return
		}
		var (
			names    []string
			versions []string
			flags    []int64
		)
		for _, d := range it {
			names = append(names, d.Package)
			versions = append(versions, d.Version)
			flags = append(flags, getDependencyFlag(d.Constraint))
		}
		writeStringArrayEntry(index, store, int32(nameTag), fieldStrArray, names)
		writeStringArrayEntry(index, store, int32(versionTag), fieldStrArray, versions)
		writeIntArrayEntry(index, store, int32(flagTag), fieldInt32, flags)
	}
	writeDeps(p.Provides(), rpmTagProvideName, rpmTagProvideVersion, rpmTagProvideFlags)
	writeDeps(p.Requires(), rpmTagRequireName, rpmTagRequireVersion, rpmTagRequireFlags)
	writeDeps(p.Conflicts(), rpmTagConflictName, rpmTagConflictVersion, rpmTagConflictFlags)
	writeDeps(p.Enhances(), rpmTagEnhanceName, rpmTagEnhanceVersion, rpmTagEnhanceVersion)
	writeDeps(p.Recommends(), rpmTagRecommendName, rpmTagRecommendVersion, rpmTagRecommendFlags)
	writeDeps(p.Suggests(), rpmTagSuggestName, rpmTagSuggestVersion, rpmTagSuggestFlags)
	return nil
}

const (
	rpmFlagDependsAny     = 0
	rpmFlagDependsLess    = 1 << 1
	rpmFlagDependsGreater = 1 << 2
	rpmFlagDependsEqual   = 1 << 3
)

func getDependencyFlag(str string) int64 {
	switch str {
	case packfile.ConstraintEq:
		return rpmFlagDependsEqual
	case packfile.ConstraintGt:
		return rpmFlagDependsGreater
	case packfile.ConstraintGe:
		return rpmFlagDependsGreater | rpmFlagDependsEqual
	case packfile.ConstraintLt:
		return rpmFlagDependsLess
	case packfile.ConstraintLe:
		return rpmFlagDependsLess | rpmFlagDependsEqual
	default:
		return rpmFlagDependsAny
	}
}

func prepareScripts(p *packfile.Package, index, store *bytes.Buffer) error {
	writeStringEntry(index, store, rpmTagPrein, fieldString, p.PreInst)
	writeStringEntry(index, store, rpmTagPostin, fieldString, p.PostInst)
	writeStringEntry(index, store, rpmTagPreun, fieldString, p.PreRem)
	writeStringEntry(index, store, rpmTagPostun, fieldString, p.PostRem)
	writeStringEntry(index, store, rpmTagCheckScript, fieldString, p.CheckScript)
	return nil
}

func prepareChanges(p *packfile.Package, index, store *bytes.Buffer) error {
	var (
		changeTime []int64
		changeName []string
		changeDesc []string
	)
	for _, c := range p.Changes {
		changeTime = append(changeTime, c.When.Unix())
		changeName = append(changeName, c.Maintainer.Name+" - "+c.Version)
		changeDesc = append(changeDesc, c.Summary)
	}
	writeIntArrayEntry(index, store, rpmTagChangeTime, fieldInt32, changeTime)
	writeStringArrayEntry(index, store, rpmTagChangeName, fieldStrArray, changeName)
	writeStringArrayEntry(index, store, rpmTagChangeText, fieldStrArray, changeDesc)
	return nil
}

func writeFiles(p *packfile.Package) (*os.File, error) {
	f, err := os.Create(p.PackageName() + ".cpio.gz")
	if err != nil {
		return nil, err
	}

	z, _ := gzip.NewWriterLevel(f, gzip.BestCompression)

	cp := cpio.NewWriter(z)
	defer func() {
		cp.Close()
		z.Flush()
		z.Close()
	}()

	slices.SortFunc(p.Files, func(a, b packfile.Resource) int {
		return strings.Compare(a.Target, b.Target)
	})
	for i, r := range p.Files {
		h := tape.Header{
			Filename: "./" + strings.ReplaceAll(r.Target, "\\", "/"),
			Mode:     r.Perm,
			Size:     r.Size,
			Uid:      0,
			Gid:      0,
			ModTime:  r.Lastmod,
		}
		if err := cp.WriteHeader(&h); err != nil {
			f.Close()
			return nil, err
		}
		sum := md5.New()
		if _, err := io.Copy(io.MultiWriter(cp, sum), r.Local); err != nil {
			f.Close()
			return nil, err
		}
		r.Local.Close()
		r.Hash = fmt.Sprintf("%+x", sum.Sum(nil))
		p.Files[i] = r
	}
	return f, nil
}

var rpmMagic = []byte{0xed, 0xab, 0xee, 0xdb}

const (
	rpmMajor    = 3
	rpmMinor    = 0
	rpmBinary   = 0
	rpmSigType  = 5
	rpmEntryLen = 16
	rpmLeadLen  = 96
)

var rpmHeader = []byte{0x8e, 0xad, 0xe8, 0x01, 0x00, 0x00, 0x00, 0x00}

const (
	rpmPayloadFormat     = "cpio"
	rpmPayloadCompressor = "gzip"
	rpmPayloadFlags      = "9"
)

const (
	rpmFileConf         = 1 << 0
	rpmFileDoc          = 1 << 1
	rpmFileAllowMissing = 1 << 3
	rpmFileNoReplace    = 1 << 4
	rpmFileGhost        = 1 << 6
	rpmFileLicense      = 1 << 7
	rpmFileReadme       = 1 << 8
	rpmFileDir          = 1 << 12
)

const (
	rpmTagSignatureIndex = 62
	rpmTagImmutableIndex = 63
)

const (
	rpmSigBase = 256
	// rpmSigPGP     = 1002
	// rpmSigDSA     = rpmSigBase + 11
	// rpmSigRSA     = rpmSigBase + 12
	rpmSigSha1    = rpmSigBase + 13
	rpmSigSha256  = rpmSigBase + 17
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
	rpmTagRpmVersion   = 1064
	rpmTagPlatform     = 1132
	// rpmTagFilenames = 5000
)

const (
	rpmTagChangeTime = 1080
	rpmTagChangeName = 1081
	rpmTagChangeText = 1082
)

const (
	rpmTagFileSizes      = 1028
	rpmTagFileModes      = 1030
	rpmTagFileDevs       = 1033
	rpmTagFileTimes      = 1034
	rpmTagFileDigests    = 1035
	rpmTagFileLinks      = 1036
	rpmTagFileFlags      = 1037
	rpmTagOwners         = 1039
	rpmTagGroups         = 1040
	rpmTagArchiveSize    = 1046
	rpmTagFileInodes     = 1096
	rpmTagFileLangs      = 1097
	rpmTagDirIndexes     = 1116
	rpmTagBasenames      = 1117
	rpmTagDirnames       = 1118
	rpmTagFileRequire    = 5002
	rpmTagFileProvide    = 5001
	rpmTagFileDigestAlgo = 5011
)

const (
	rpmTagProvideName      = 1047
	rpmTagProvideVersion   = 1113
	rpmTagProvideFlags     = 1112
	rpmTagRequireName      = 1049
	rpmTagRequireVersion   = 1050
	rpmTagRequireFlags     = 1048
	rpmTagConflictName     = 1054
	rpmTagConflictVersion  = 1055
	rpmTagConflictFlags    = 1053
	rpmTagObsoleteName     = 1090
	rpmTagObsoleteVersion  = 1115
	rpmTagObsoleteFlags    = 1114
	rpmTagEnhanceName      = 5055
	rpmTagEnhanceVersion   = 5056
	rpmTagEnhanceFlags     = 5057
	rpmTagRecommendName    = 5046
	rpmTagRecommendVersion = 5047
	rpmTagRecommendFlags   = 5048
	rpmTagSuggestName      = 5049
	rpmTagSuggestVersion   = 5050
	rpmTagSuggestFlags     = 5051
)

const (
	rpmTagTrans            = 1151
	rpmTagTransFlags       = 5024
	rpmTagTransProg        = 1153
	rpmTagPrein            = 1023
	rpmTagPreinFlags       = 5020
	rpmTagPreinProg        = 1085
	rpmTagPostin           = 1024
	rpmTagPostinFlags      = 5021
	rpmTagPostinProg       = 1086
	rpmTagPostun           = 1026
	rpmTagPostunFlags      = 5023
	rpmTagPostunProg       = 1088
	rpmTagPreun            = 1025
	rpmTagPreunFlags       = 5022
	rpmTagPreunProg        = 1087
	rpmTagCheckScript      = 1079
	rpmTagCheckScriptFlags = 5026
	rpmTagCheckScriptProg  = 1091
)

const (
	fieldNull int32 = iota
	fieldChar
	fieldInt8
	fieldInt16
	fieldInt32
	fieldInt64
	fieldString
	fieldBinary
	fieldStrArray
	fieldI18NString
)

const (
	rpmDigestMd5    = 1
	rpmDigestSha1   = 2
	rpmDigestSha256 = 8
	rpmDigestSha384 = 9
	rpmDigestSha512 = 10
)

func writeHashBinary(idx, str *bytes.Buffer, tag, kind int32, val hash.Hash) error {
	sum := val.Sum(nil)
	return writeBinaryEntry(idx, str, tag, kind, sum[:])
}

func writeHashString(idx, str *bytes.Buffer, tag, kind int32, val hash.Hash) error {
	sum := hex.EncodeToString(val.Sum(nil))
	return writeStringEntry(idx, str, tag, kind, sum)
}

func writeBinaryEntry(idx, str *bytes.Buffer, tag, kind int32, val []byte) error {
	if len(val) == 0 {
		return nil
	}
	binary.Write(idx, binary.BigEndian, tag)
	binary.Write(idx, binary.BigEndian, kind)
	binary.Write(idx, binary.BigEndian, int32(str.Len()))
	binary.Write(idx, binary.BigEndian, int32(len(val)))

	str.Write(val)
	return nil
}

func writeStringEntry(idx, str *bytes.Buffer, tag, kind int32, val string) error {
	if val == "" {
		return nil
	}
	binary.Write(idx, binary.BigEndian, tag)
	binary.Write(idx, binary.BigEndian, kind)
	binary.Write(idx, binary.BigEndian, int32(str.Len()))
	binary.Write(idx, binary.BigEndian, int32(1))

	_, err := io.WriteString(str, val+"\x00")
	return err
}

func writeStringArrayEntry(idx, str *bytes.Buffer, tag, kind int32, val []string) error {
	if len(val) == 0 {
		return nil
	}
	binary.Write(idx, binary.BigEndian, tag)
	binary.Write(idx, binary.BigEndian, kind)
	binary.Write(idx, binary.BigEndian, int32(str.Len()))
	binary.Write(idx, binary.BigEndian, int32(len(val)))

	for i := range val {
		_, err := io.WriteString(str, val[i]+"\x00")
		if err != nil {
			return err
		}
	}
	return nil
}

func writeIntEntry(idx, str *bytes.Buffer, tag, kind int32, val int64) error {
	if err := fillToBoundary(str, kind); err != nil {
		return err
	}

	binary.Write(idx, binary.BigEndian, tag)
	binary.Write(idx, binary.BigEndian, kind)
	binary.Write(idx, binary.BigEndian, int32(str.Len()))
	binary.Write(idx, binary.BigEndian, int32(1))

	switch kind {
	case fieldInt8:
		binary.Write(str, binary.BigEndian, int8(val))
	case fieldInt16:
		binary.Write(str, binary.BigEndian, int16(val))
	case fieldInt32:
		binary.Write(str, binary.BigEndian, int32(val))
	case fieldInt64:
		binary.Write(str, binary.BigEndian, val)
	default:
	}
	return nil
}

func writeIntArrayEntry(idx, str *bytes.Buffer, tag, kind int32, val []int64) error {
	if len(val) == 0 {
		return nil
	}

	if err := fillToBoundary(str, kind); err != nil {
		return err
	}

	binary.Write(idx, binary.BigEndian, tag)
	binary.Write(idx, binary.BigEndian, kind)
	binary.Write(idx, binary.BigEndian, int32(str.Len()))
	binary.Write(idx, binary.BigEndian, int32(len(val)))

	for i := range val {
		switch kind {
		case fieldInt8:
			binary.Write(str, binary.BigEndian, int8(val[i]))
		case fieldInt16:
			binary.Write(str, binary.BigEndian, int16(val[i]))
		case fieldInt32:
			binary.Write(str, binary.BigEndian, int32(val[i]))
		case fieldInt64:
			binary.Write(str, binary.BigEndian, val[i])
		default:
		}
	}
	return nil
}

func fillToBoundary(str *bytes.Buffer, kind int32) error {
	var fill []byte

	switch offset := str.Len(); kind {
	case fieldInt8:
	case fieldInt16:
		if mod := offset % 2; mod != 0 {
			fill = append(fill, 0)
		}
	case fieldInt32:
		if mod := offset % 4; mod != 0 {
			fill = make([]byte, 4-mod)
		}
	case fieldInt64:
		if mod := offset % 8; mod != 0 {
			fill = make([]byte, 8-mod)
		}
	default:
		return fmt.Errorf("invalid type: only number type accepted")
	}
	if len(fill) > 0 {
		str.Write(fill)
	}
	return nil
}
