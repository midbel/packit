package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

func main() {
	var (
		printHeader   = flag.Bool("h", false, "print ELF header")
		printSections = flag.Bool("s", false, "print section headers")
		printSegments = flag.Bool("p", false, "print segments headers")
	)
	flag.Parse()

	file, err := Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer file.Close()

	switch {
	case *printHeader:
		printELF(file.ELFHeader)
	case *printSegments:
		var size int64
		slices.SortFunc(file.Programs, func(a, b ProgramHeader) int {
			return int(a.Offset) - int(b.Offset)
		})
		for _, ph := range file.Programs {
			// fmt.Printf("%#8x %#08x -> %#08x %8d\n", ph.Type, ph.Offset, ph.Offset+ph.SegmentSizeFile, ph.SegmentSizeFile)
			fmt.Printf("%#8x %d -> %d %8d\n", ph.Type, ph.Offset, ph.Offset+ph.SegmentSizeFile, ph.SegmentSizeFile)
			size += int64(ph.SegmentSizeFile)
		}
		fmt.Printf("segments size: %d (bytes)\n", file.PhSize*file.PhCount)
		fmt.Printf("total size   : %d (bytes)\n", size)
	case *printSections:
		var size int64
		slices.SortFunc(file.Sections, func(a, b SectionHeader) int {
			return int(a.Offset) - int(b.Offset)
		})
		for i, sh := range file.Sections {
			// fmt.Printf("[%2d] %-24s %-12s %#8x -> %#8x %8d => %d\n", i, sh.Label, getSectionTypeName(sh), sh.Offset, sh.Offset+sh.Size, sh.Size, sh.Link)
			fmt.Printf("[%2d] %-24s %-12s %d -> %d %8d => %d\n", i, sh.Label, getSectionTypeName(sh), sh.Offset, sh.Offset+sh.Size, sh.Size, sh.Link)
			size += int64(sh.Size)
		}
		fmt.Printf("section size: %d (bytes)\n", file.ShSize*file.ShCount)
		fmt.Printf("total size  : %d (bytes)\n", size)
	default:
		fmt.Printf("dependencies of %s\n", flag.Arg(0))
		for _, i := range file.Libs() {
			fmt.Printf("- %s\n", i)
		}
	}
	fmt.Println(">>> size:", file.TotalSize())
	fmt.Println(">>> static:", file.Static())
	fmt.Println(">>> linker:", file.Linker())
	if err := file.Strip(flag.Arg(0) + ".out"); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

const (
	Arch32 = 1
	Arch64 = 2
)

var magic = []byte{0x7F, 0x45, 0x4c, 0x46}

type File struct {
	*ELFHeader
	file *os.File
}

func Open(file string) (*File, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	hdr, err := load(r)
	if err != nil {
		return nil, err
	}
	elf := File{
		ELFHeader: hdr,
		file:      r,
	}
	return &elf, nil
}

func (f *File) TotalSize() int64 {
	var size int64
	size += int64(f.Size)
	size += int64(f.PhSize) * int64(f.PhCount)
	size += int64(f.ShSize) * int64(f.ShCount)

	for i := range f.Programs {
		size += int64(f.Programs[i].SegmentSizeFile)
	}
	for i := range f.Sections {
		size += int64(f.Sections[i].Size)
	}
	return size
}

func (f *File) Strip(target string) error {
	if target == "" {
		target = f.file.Name() + ".strip"
	}
	w, err := os.Create(target)
	if err != nil {
		return err
	}
	defer w.Close()

	var (
		sections  = f.getStrippableSections()
		totalSize int64
	)
	totalSize += int64(f.Size)
	totalSize += int64(f.PhSize) * int64(f.PhCount)
	for i := range f.Programs {
		totalSize += int64(f.Programs[i].SegmentSizeFile)
	}
	totalSize += int64(f.ShSize) * int64(len(sections))
	for i := range sections {
		totalSize += int64(sections[i].Size)
	}
	if err := w.Truncate(totalSize); err != nil {
		return err
	}

	var tmp bytes.Buffer

	namesIndex := slices.IndexFunc(sections, func(sh SectionHeader) bool {
		return sh.Label == ".shstrtab"
	})
	if namesIndex < 0 {
		return fmt.Errorf("missing shstrtab section")
	}

	tmp.Write(magic)
	binary.Write(&tmp, binary.BigEndian, f.Class)
	binary.Write(&tmp, binary.BigEndian, f.Endianness)
	binary.Write(&tmp, binary.BigEndian, f.Version)
	binary.Write(&tmp, binary.BigEndian, f.AbiOs)
	binary.Write(&tmp, binary.BigEndian, f.AbiVersion)

	tmp.Write(make([]byte, 7))

	binary.Write(&tmp, f.ByteOrder(), f.Type)
	binary.Write(&tmp, f.ByteOrder(), f.Machine)
	binary.Write(&tmp, f.ByteOrder(), f.ElfVersion)

	if f.Is32() {
		binary.Write(&tmp, f.ByteOrder(), uint32(f.EntryAddr))
		binary.Write(&tmp, f.ByteOrder(), uint32(f.ProgramAddr))
		binary.Write(&tmp, f.ByteOrder(), uint32(f.SectionAddr))
	} else {
		binary.Write(&tmp, f.ByteOrder(), f.EntryAddr)
		binary.Write(&tmp, f.ByteOrder(), f.ProgramAddr)
		binary.Write(&tmp, f.ByteOrder(), f.SectionAddr)
	}
	binary.Write(&tmp, f.ByteOrder(), f.Flags)
	binary.Write(&tmp, f.ByteOrder(), f.Size)
	binary.Write(&tmp, f.ByteOrder(), f.PhSize)
	binary.Write(&tmp, f.ByteOrder(), f.PhCount)
	binary.Write(&tmp, f.ByteOrder(), f.ShSize)
	binary.Write(&tmp, f.ByteOrder(), uint16(len(sections)))
	binary.Write(&tmp, f.ByteOrder(), uint16(namesIndex))

	if _, err := io.Copy(w, &tmp); err != nil {
		return err
	}

	return nil
}

func (f *File) Names() []string {
	var list []string
	for _, sh := range f.Sections {
		list = append(list, sh.Label)
	}
	return list
}

func (f *File) Static() bool {
	ix := slices.IndexFunc(f.Sections, func(sh SectionHeader) bool {
		return sh.Label == ".dynamic" || sh.Label == ".interp"
	})
	return ix < 0
}

func (f *File) Linker() string {
	name, _ := f.getInterp()
	return name
}

func (f *File) Libs() []string {
	entries, err := f.getDynEntries()
	if err != nil {
		return nil
	}
	return f.getLibraries(entries)
}

func (f *File) Close() error {
	return f.file.Close()
}

func (f *File) getLibraries(entries []DynamicEntry) []string {
	ix := slices.IndexFunc(f.Sections, func(h SectionHeader) bool {
		return h.Label == ".dynstr"
	})
	if ix < 0 {
		return nil
	}
	sh := f.Sections[ix]
	var (
		buf    = make([]byte, sh.Size)
		rs     = io.NewSectionReader(f.file, int64(sh.Offset), int64(sh.Size))
		offset int
		list   = make(map[uint32]string)
	)
	if _, err := io.ReadFull(rs, buf); err != nil {
		return nil
	}
	for {
		x := bytes.IndexByte(buf[offset:], 0)
		if x < 0 {
			break
		}
		str := buf[offset : offset+x]
		list[uint32(offset)] = string((str))
		offset += x + 1
	}

	var needed []string
	for _, e := range entries {
		if e.Tag != 1 {
			continue
		}
		needed = append(needed, list[uint32(e.Value)])
	}
	return needed
}

func (f *File) getInterp() (string, error) {
	ix := slices.IndexFunc(f.Sections, func(sh SectionHeader) bool {
		return sh.Label == ".interp"
	})
	if ix < 0 {
		return "", fmt.Errorf(".interp section not found")
	}
	var (
		sh  = f.Sections[ix]
		rs  = io.NewSectionReader(f.file, int64(sh.Offset), int64(sh.Size))
		buf = make([]byte, sh.Size)
	)
	if _, err := io.ReadFull(rs, buf); err != nil {
		return "", err
	}
	return string(buf[:len(buf)-1]), nil
}

func (f *File) getDynEntries() ([]DynamicEntry, error) {
	ix := slices.IndexFunc(f.Sections, func(sh SectionHeader) bool {
		return sh.Label == ".dynamic"
	})
	if ix < 0 {
		return nil, fmt.Errorf("dynamic section not found")
	}
	var (
		sh    = f.Sections[ix]
		rs    = io.NewSectionReader(f.file, int64(sh.Offset), int64(sh.Size))
		count = int(sh.Size) / 16
	)
	var list []DynamicEntry
	for i := 0; i < count; i++ {
		var e DynamicEntry
		binary.Read(rs, f.ByteOrder(), &e.Tag)
		binary.Read(rs, f.ByteOrder(), &e.Value)
		list = append(list, e)
	}
	return list, nil
}

func (f *File) getNames() []string {
	var (
		sh     = f.Sections[f.NamesIndex]
		buf    = make([]byte, sh.Size)
		rs     = io.NewSectionReader(f.file, int64(sh.Offset), int64(sh.Size))
		list   []string
		offset int
	)
	if _, err := io.ReadFull(rs, buf); err != nil {
		return nil
	}
	for {
		x := bytes.IndexByte(buf[offset:], 0)
		if x < 0 {
			break
		}
		str := buf[offset : offset+x]
		list = append(list, string(str))
		offset += x + 1
	}
	return list
}

func (f *File) getStrippableSections() []SectionHeader {
	var list []SectionHeader
	for _, sh := range f.Sections {
		if sh.canStrip() {
			continue
		}
		list = append(list, sh)
	}
	return list
}

type ELFHeader struct {
	Class       uint8
	Endianness  uint8
	Version     uint8
	AbiOs       uint8
	AbiVersion  uint8
	Type        uint16
	Machine     uint16
	EntryAddr   uint64
	ProgramAddr uint64
	SectionAddr uint64
	ElfVersion  uint32
	Flags       uint32
	Size        uint16
	PhSize      uint16
	PhCount     uint16
	ShSize      uint16
	ShCount     uint16
	NamesIndex  uint16

	Programs []ProgramHeader
	Sections []SectionHeader
}

func (e ELFHeader) Is32() bool {
	return e.Class == Arch32
}

func (e ELFHeader) Is64() bool {
	return e.Class == Arch64
}

func (e ELFHeader) ByteOrder() binary.ByteOrder {
	if e.Endianness == 1 {
		return binary.LittleEndian
	}
	return binary.BigEndian
}

type DynamicEntry struct {
	Tag   uint64
	Value uint64
}

type ProgramHeader struct {
	Type            uint32
	Flags           uint32
	Offset          uint64
	VirtualAddr     uint64
	PhysicalAddr    uint64
	SegmentSizeFile uint64
	SegmentSizeMem  uint64
	Alignment       uint64
}

type SectionHeader struct {
	Label     string
	Name      uint32
	Type      uint32
	Flags     uint64
	Addr      uint64
	Offset    uint64
	Size      uint64
	Link      uint32
	Info      uint32
	AddrAlign uint64
	EntSize   uint64
}

func (s SectionHeader) canStrip() bool {
	if strings.HasPrefix(s.Label, ".debug") {
		return true
	}
	if strings.HasPrefix(s.Label, ".note") {
		return true
	}
	return s.Label == ".comment" || s.Label == ".symtab" || s.Label == ".strtab"
}

func getSectionTypeName(sh SectionHeader) string {
	switch sh.Type {
	case 0x00:
		return "NULL"
	case 0x01:
		return "PROGBITS"
	case 0x02:
		return "SYMTAB"
	case 0x03:
		return "STRTAB"
	case 0x04:
		return "RELA"
	case 0x05:
		return "HASH"
	case 0x06:
		return "DYNAMIC"
	case 0x07:
		return "NOTE"
	case 0x08:
		return "NOBITS"
	case 0x09:
		return "REL"
	case 0x0a:
		return "SHLIB"
	case 0x0b:
		return "DYNSYM"
	default:
		return "other"
	}
}

func readNames(elf *ELFHeader, r io.ReaderAt) (map[uint32]string, error) {
	var (
		ns     = elf.Sections[elf.NamesIndex]
		buf    = make([]byte, ns.Size)
		rs     = io.NewSectionReader(r, int64(ns.Offset), int64(ns.Size))
		list   = make(map[uint32]string)
		offset int
	)
	if _, err := io.ReadFull(rs, buf); err != nil {
		return nil, err
	}
	for {
		x := bytes.IndexByte(buf[offset:], 0)
		if x < 0 {
			break
		}
		str := buf[offset : offset+x]
		list[uint32(offset)] = string((str))
		offset += x + 1
	}
	return list, nil
}

func printELF(elf *ELFHeader) {
	fmt.Printf("Class                      : %s\n", getClassName(elf))
	fmt.Printf("Data                       : %s\n", getEndiannessName(elf))
	fmt.Printf("Version                    : %s\n", getVersionName(elf))
	fmt.Printf("OS/ABI                     : %s\n", getAbiOS(elf))
	fmt.Printf("OS/Version                 : %d\n", elf.AbiVersion)
	fmt.Printf("Type                       : %s\n", getTypeName(elf))
	fmt.Printf("Machine                    : %s\n", getMachineName(elf))
	fmt.Printf("Version                    : %#x\n", elf.Version)
	fmt.Printf("Entry point address        : %#x\n", elf.EntryAddr)
	fmt.Printf("Start of program headers   : %d\n", elf.ProgramAddr)
	fmt.Printf("Start of section headers   : %d\n", elf.SectionAddr)
	fmt.Printf("Size of ELF Header         : %d\n", elf.Size)
	fmt.Printf("Number of program headers  : %d\n", elf.PhCount)
	fmt.Printf("Size of program headers    : %d\n", elf.PhSize)
	fmt.Printf("Number of section headers  : %d\n", elf.ShCount)
	fmt.Printf("Size of section headers    : %d\n", elf.ShSize)
	fmt.Printf("Section header string index: %d\n", elf.NamesIndex)
}

func load(r *os.File) (*ELFHeader, error) {
	elf, err := readHeader(r)
	if err != nil {
		return nil, err
	}
	r.Seek(int64(elf.ProgramAddr), io.SeekStart)
	for i := 0; i < int(elf.PhCount); i++ {
		if err := readProgramHeader(elf, r); err != nil {
			return nil, err
		}
	}
	r.Seek(int64(elf.SectionAddr), io.SeekStart)
	for i := 0; i < int(elf.ShCount); i++ {
		if err := readSectionHeader(elf, r); err != nil {
			return nil, err
		}
	}
	names, err := readNames(elf, r)
	for i, sh := range elf.Sections {
		elf.Sections[i].Label = names[sh.Name]
	}
	return elf, nil
}

func readSectionHeader(elf *ELFHeader, r io.Reader) error {
	var sh SectionHeader

	binary.Read(r, elf.ByteOrder(), &sh.Name)
	binary.Read(r, elf.ByteOrder(), &sh.Type)

	if elf.Is32() {
		var (
			flags     uint32
			addr      uint32
			offset    uint32
			size      uint32
			link      uint32
			info      uint32
			addrAlign uint32
			entSize   uint32
		)
		binary.Read(r, elf.ByteOrder(), &flags)
		binary.Read(r, elf.ByteOrder(), &addr)
		binary.Read(r, elf.ByteOrder(), &offset)
		binary.Read(r, elf.ByteOrder(), &size)
		binary.Read(r, elf.ByteOrder(), &link)
		binary.Read(r, elf.ByteOrder(), &info)
		binary.Read(r, elf.ByteOrder(), &addrAlign)
		binary.Read(r, elf.ByteOrder(), &entSize)

		sh.Flags = uint64(flags)
		sh.Addr = uint64(addr)
		sh.Offset = uint64(offset)
		sh.Size = uint64(size)
		sh.Link = link
		sh.Info = info
		sh.AddrAlign = uint64(addrAlign)
		sh.EntSize = uint64(entSize)
	} else {
		binary.Read(r, elf.ByteOrder(), &sh.Flags)
		binary.Read(r, elf.ByteOrder(), &sh.Addr)
		binary.Read(r, elf.ByteOrder(), &sh.Offset)
		binary.Read(r, elf.ByteOrder(), &sh.Size)
		binary.Read(r, elf.ByteOrder(), &sh.Link)
		binary.Read(r, elf.ByteOrder(), &sh.Info)
		binary.Read(r, elf.ByteOrder(), &sh.AddrAlign)
		binary.Read(r, elf.ByteOrder(), &sh.EntSize)
	}
	elf.Sections = append(elf.Sections, sh)
	return nil
}

func readProgramHeader(elf *ELFHeader, r io.Reader) error {
	var ph ProgramHeader
	binary.Read(r, elf.ByteOrder(), &ph.Type)
	if elf.Is32() {
		var (
			offset       uint32
			virtualAddr  uint32
			physicalAddr uint32
			sizeFile     uint32
			sizeMem      uint32
			flags        uint32
			align        uint32
		)
		binary.Read(r, elf.ByteOrder(), &offset)
		binary.Read(r, elf.ByteOrder(), &virtualAddr)
		binary.Read(r, elf.ByteOrder(), &physicalAddr)
		binary.Read(r, elf.ByteOrder(), &sizeFile)
		binary.Read(r, elf.ByteOrder(), &sizeMem)
		binary.Read(r, elf.ByteOrder(), &flags)
		binary.Read(r, elf.ByteOrder(), &align)

		ph.Offset = uint64(offset)
		ph.VirtualAddr = uint64(virtualAddr)
		ph.PhysicalAddr = uint64(physicalAddr)
		ph.SegmentSizeFile = uint64(sizeFile)
		ph.SegmentSizeMem = uint64(sizeMem)
		ph.Alignment = uint64(align)
		ph.Flags = flags
	} else {
		binary.Read(r, elf.ByteOrder(), &ph.Flags)
		binary.Read(r, elf.ByteOrder(), &ph.Offset)
		binary.Read(r, elf.ByteOrder(), &ph.VirtualAddr)
		binary.Read(r, elf.ByteOrder(), &ph.PhysicalAddr)
		binary.Read(r, elf.ByteOrder(), &ph.SegmentSizeFile)
		binary.Read(r, elf.ByteOrder(), &ph.SegmentSizeMem)
		binary.Read(r, elf.ByteOrder(), &ph.Alignment)
	}
	elf.Programs = append(elf.Programs, ph)
	return nil
}

func readHeader(rs io.Reader) (*ELFHeader, error) {
	var (
		elf ELFHeader
		err error
		buf = make([]byte, 4)
	)

	if _, err = io.ReadFull(rs, buf); err != nil {
		return nil, err
	}
	if !bytes.Equal(buf, magic) {
		return nil, fmt.Errorf("invalid magic %x", buf)
	}

	binary.Read(rs, binary.BigEndian, &elf.Class)
	binary.Read(rs, binary.BigEndian, &elf.Endianness)
	binary.Read(rs, binary.BigEndian, &elf.Version)
	binary.Read(rs, binary.BigEndian, &elf.AbiOs)
	binary.Read(rs, binary.BigEndian, &elf.AbiVersion)

	if _, err = io.CopyN(io.Discard, rs, 7); err != nil {
		return nil, err
	}

	binary.Read(rs, elf.ByteOrder(), &elf.Type)
	binary.Read(rs, elf.ByteOrder(), &elf.Machine)
	binary.Read(rs, elf.ByteOrder(), &elf.ElfVersion)
	if elf.Is32() {
		var (
			entAddr  uint32
			progAddr uint32
			sectAddr uint32
		)
		binary.Read(rs, elf.ByteOrder(), &entAddr)
		binary.Read(rs, elf.ByteOrder(), &progAddr)
		binary.Read(rs, elf.ByteOrder(), &sectAddr)

		elf.EntryAddr = uint64(entAddr)
		elf.ProgramAddr = uint64(progAddr)
		elf.SectionAddr = uint64(sectAddr)
	} else {
		binary.Read(rs, elf.ByteOrder(), &elf.EntryAddr)
		binary.Read(rs, elf.ByteOrder(), &elf.ProgramAddr)
		binary.Read(rs, elf.ByteOrder(), &elf.SectionAddr)
	}

	binary.Read(rs, elf.ByteOrder(), &elf.Flags)
	binary.Read(rs, elf.ByteOrder(), &elf.Size)
	binary.Read(rs, elf.ByteOrder(), &elf.PhSize)
	binary.Read(rs, elf.ByteOrder(), &elf.PhCount)
	binary.Read(rs, elf.ByteOrder(), &elf.ShSize)
	binary.Read(rs, elf.ByteOrder(), &elf.ShCount)
	binary.Read(rs, elf.ByteOrder(), &elf.NamesIndex)

	return &elf, nil
}

func getAbiOS(elf *ELFHeader) string {
	return "xxx"
}

func getTypeName(elf *ELFHeader) string {
	switch elf.Type {
	case 0x00:
		return "unknown"
	case 0x01:
		return "relocatable file"
	case 0x02:
		return "executable file"
	case 0x03:
		return "shared object"
	case 0x04:
		return "core file"
	default:
		return "other"
	}
}

func getMachineName(elf *ELFHeader) string {
	return "xxx"
}

func getClassName(elf *ELFHeader) string {
	if elf.Is32() {
		return "ELF32"
	}
	return "ELF64"
}

func getEndiannessName(elf *ELFHeader) string {
	if elf.Endianness == 1 {
		return "little endian"
	}
	return "big endian"
}

func getVersionName(elf *ELFHeader) string {
	if elf.Version == 1 {
		return "current"
	}
	return ""
}
