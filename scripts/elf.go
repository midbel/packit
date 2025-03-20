package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
)

func main() {
	var (
		printHeader   = flag.Bool("h", false, "print ELF header")
		printSections = flag.Bool("s", false, "print section headers")
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
	case *printSections:
		for i, sh := range file.Sections {
			fmt.Printf("[%2d] %-24s %-12s %#8x %8d\n", i, sh.Label, getSectionTypeName(sh), sh.Offset, sh.Size)
		}
	default:
		fmt.Printf("dependencies of %s\n", flag.Arg(0))
		for _, i := range file.Libs() {
			fmt.Printf("- %s\n", i)
		}
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
		return  nil, err
	}
	hdr, err := load(r)
	if err != nil {
		return nil, err
	}
	elf := File{
		ELFHeader: hdr,
		file: r,
	}
	return &elf, nil
}

func (f *File) Strip() (*File, error) {
	return nil, nil
}

func (f *File) Names() []string {
	var list []string
	for _, sh := range f.Sections {
		list = append(list, sh.Label)
	}
	return list
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
	Type            int32
	Flags           int32
	Offset          int64
	VirtualAddr     int64
	PhysicalAddr    int64
	SegmentSizeFile int64
	SegmentSizeMem  int64
	Alignment       int64
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
			offset       int32
			virtualAddr  int32
			physicalAddr int32
			sizeFile     int32
			sizeMem      int32
			flags        int32
			align        int32
		)
		binary.Read(r, elf.ByteOrder(), &offset)
		binary.Read(r, elf.ByteOrder(), &virtualAddr)
		binary.Read(r, elf.ByteOrder(), &physicalAddr)
		binary.Read(r, elf.ByteOrder(), &sizeFile)
		binary.Read(r, elf.ByteOrder(), &sizeMem)
		binary.Read(r, elf.ByteOrder(), &flags)
		binary.Read(r, elf.ByteOrder(), &align)

		ph.Offset = int64(offset)
		ph.VirtualAddr = int64(virtualAddr)
		ph.PhysicalAddr = int64(physicalAddr)
		ph.SegmentSizeFile = int64(sizeFile)
		ph.SegmentSizeMem = int64(sizeMem)
		ph.Alignment = int64(align)
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