package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
)

func main() {
	var (
		orderBy       = flag.String("o", "", "order headers by given field")
		printNames    = flag.Bool("n", false, "print list of section names")
		printHeader   = flag.Bool("h", false, "print ELF header")
		printSections = flag.Bool("s", false, "print section headers")
		printSegments = flag.Bool("p", false, "print segments headers")
		strippedFile  = flag.Bool("strip", false, "stripped given file")
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
		printELF(file)
	case *printNames:
		for _, n := range file.Names() {
			fmt.Println(n)
		}
	case *printSegments:
		printFileSegments(file, *orderBy)
	case *printSections:
		printFileSections(file, *orderBy)
	case *strippedFile:
		if err := file.Strip(flag.Arg(0) + ".out"); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	default:
		fmt.Printf("dependencies of %s\n", flag.Arg(0))
		for _, i := range file.Libs() {
			fmt.Printf("- %s\n", i)
		}
	}
}

func printELF(file *File) {
	fmt.Printf("Class                      : %s\n", getClassName(file.ELFHeader))
	fmt.Printf("Data                       : %s\n", getEndiannessName(file.ELFHeader))
	fmt.Printf("Version                    : %s\n", getVersionName(file.ELFHeader))
	fmt.Printf("OS/ABI                     : %s\n", getAbiOS(file.ELFHeader))
	fmt.Printf("OS/Version                 : %d\n", file.AbiVersion)
	fmt.Printf("Type                       : %s\n", getTypeName(file.ELFHeader))
	fmt.Printf("Machine                    : %s\n", getMachineName(file.ELFHeader))
	fmt.Printf("Version                    : %#x\n", file.Version)
	fmt.Printf("Entry point address        : %#x\n", file.EntryAddr)
	fmt.Printf("Start of program headers   : %d\n", file.ProgramAddr)
	fmt.Printf("Start of section headers   : %d\n", file.SectionAddr)
	fmt.Printf("Size of ELF Header         : %d\n", file.Size)
	fmt.Printf("Number of program headers  : %d\n", file.PhCount)
	fmt.Printf("Size of program headers    : %d\n", file.PhSize)
	fmt.Printf("Number of section headers  : %d\n", file.ShCount)
	fmt.Printf("Size of section headers    : %d\n", file.ShSize)
	fmt.Printf("Section header string index: %d\n", file.NamesIndex)
}

func printFileSections(file *File, orderBy string) {
	if orderBy != "" {
		slices.SortFunc(file.Sections, func(a, b SectionHeader) int {
			return int(a.Offset) - int(b.Offset)
		})
	}
	var size int64
	for i, sh := range file.Sections {
		var (
			starts = formatNumber(int64(sh.Offset))
			ends   = formatNumber(int64(sh.Offset + sh.Size))
			sz     = formatNumber(int64(sh.Size))
		)
		fmt.Printf("[%2d] %-24s %-12s %2d %12s -> %12s %12s => %d\n", i, sh.Label, getSectionTypeName(sh), sh.AddrAlign, starts, ends, sz, sh.Link)
		size += int64(sh.Size)
	}
	var (
		starts = formatNumber(int64(file.SectionAddr))
		ends   = formatNumber(int64(file.SectionAddr + uint64(file.ShSize*file.ShCount)))
	)
	fmt.Printf("address     : %s -> %s\n", starts, ends)
	fmt.Printf("section size: %s (bytes)\n", formatNumber(int64(file.ShSize*file.ShCount)))
	fmt.Printf("total size  : %s (bytes)\n", formatNumber(int64(size)))
}

func printFileSegments(file *File, orderBy string) {
	if orderBy != "" {
		slices.SortFunc(file.Programs, func(a, b ProgramHeader) int {
			return int(a.Offset) - int(b.Offset)
		})
	}
	var size int64
	for i, ph := range file.Programs {
		var (
			starts = formatNumber(int64(ph.Offset))
			ends   = formatNumber(int64(ph.Offset + ph.FileSize))
			sz     = formatNumber(int64(ph.FileSize))
			mz     = formatNumber(int64(ph.MemSize))
		)
		fmt.Printf("[%2d] %-12s %12s -> %12s %12s (%12s)\n", i, getProgramTypeName(ph), starts, ends, sz, mz)
		size += int64(ph.FileSize)
	}

	var (
		starts = formatNumber(int64(file.ProgramAddr))
		ends   = formatNumber(int64(file.ProgramAddr + uint64(file.PhSize*file.PhCount)))
	)
	fmt.Printf("address      : %s -> %s\n", starts, ends)
	fmt.Printf("segments size: %s (bytes)\n", formatNumber(int64(file.PhSize*file.PhCount)))
	fmt.Printf("total size   : %s (bytes)\n", formatNumber(int64(size)))
	fmt.Println()
	fmt.Println("segments sections")
	groups := getSectionsForSegments(file)
	for i, gs := range groups {
		fmt.Printf("[%02d] %s\n", i, strings.Join(gs, " "))
	}
}

func getSectionsForSegments(f *File) [][]string {
	var all [][]string
	for _, ph := range f.Programs {
		var (
			list   []string
			starts = ph.Offset
			ends   = ph.Offset + ph.MemSize
		)
		for _, sh := range f.Sections {
			if sh.Type == 0 {
				continue
			}
			if sh.Offset >= starts && sh.Offset < ends {
				list = append(list, sh.Label)
			}
		}
		all = append(all, list)
	}
	return all
}

func formatNumber(val int64) string {
	nu := strconv.FormatInt(val, 10)
	if val < 1000 {
		return nu
	}
	var (
		in     = []rune(nu)
		out    = make([]rune, len(in)+(len(in)/3))
		offset = len(out) - 1
	)
	for i, j := len(in)-1, 0; i >= 0; i-- {
		out[offset] = in[i]
		offset--
		j++
		if j == 3 && i > 0 {
			out[offset] = '_'
			offset--
			j = 0
		}
	}
	return strings.TrimLeft(string(out), "\x00")
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
	return size
}

func (f *File) Strip(target string) error {
	if target == "" {
		target = f.file.Name() + ".out"
	}
	w, err := os.Create(target)
	if err != nil {
		return err
	}
	defer w.Close()

	var (
		null   []byte
		tmp    io.Reader
		offset int64
	)
	f.Sections = f.getStrippableSections()
	if err != nil {
		return err
	}
	others := slices.Clone(f.Sections)
	slices.SortFunc(others, func(a, b SectionHeader) int {
		if a.isTLS() {
			return 1
		}
		if b.isTLS() {
			return -1
		}
		return int(a.Offset) - int(b.Offset)
	})
	for i, sh := range others {
		offset := sh.Offset
		// if i > 0 && others[i-1].Offset > 0 {
		// 	offset = others[i-1].Offset + others[i-1].Size
		// }
		if offset > 0 {
			if mod := offset % sh.AddrAlign; mod != 0 {
				offset += sh.AddrAlign - mod
			}
		}
		fmt.Printf("[%2d] %-24s: %12s => %12s\n", i, sh.Label, formatNumber(int64(offset)), formatNumber(int64(offset+sh.Size)))
		ix := slices.IndexFunc(f.Sections, func(other SectionHeader) bool {
			return other.Label == sh.Label
		})
		others[i].Offset = offset
		f.Sections[ix].Offset = offset

		r := io.NewSectionReader(f.file, int64(sh.Offset), int64(sh.Size))
		pos, err := w.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		if pos < int64(offset) {
			null = make([]byte, int64(offset)-pos)
			if _, err = w.Write(null); err != nil {
				return err
			}
		}
		if _, err = io.Copy(w, r); err != nil {
			return err
		}
	}
	offset, _ = w.Seek(0, io.SeekCurrent)
	if tmp, err = writeSectionHeaders(f); err != nil {
		return err
	}
	if _, err = io.Copy(w, tmp); err != nil {
		return err
	}
	namesIndex := slices.IndexFunc(f.Sections, func(sh SectionHeader) bool {
		return sh.Label == ".shstrtab"
	})
	x := *f
	x.ShCount = uint16(len(f.Sections))
	x.SectionAddr = uint64(offset)
	x.NamesIndex = uint16(namesIndex)
	if tmp, err = writeHeader(&x); err != nil {
		return err
	}

	if _, err = w.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err = io.Copy(w, tmp); err != nil {
		return err
	}
	for i, ph := range f.Programs {
		_, _ = i, ph
	}
	if tmp, err = writeProgramHeaders(f); err != nil {
		return err
	}
	if _, err = io.Copy(w, tmp); err != nil {
		return err
	}

	null = make([]byte, f.ShCount*f.ShSize)
	if _, err = w.Write(null); err != nil {
		return err
	}
	return err
}

func writeSectionHeaders(f *File) (io.Reader, error) {
	var tmp bytes.Buffer
	for _, sh := range f.Sections {
		binary.Write(&tmp, f.ByteOrder(), sh.Name)
		binary.Write(&tmp, f.ByteOrder(), sh.Type)
		if f.Is32() {
			binary.Write(&tmp, f.ByteOrder(), uint32(sh.Flags))
			binary.Write(&tmp, f.ByteOrder(), uint32(sh.Addr))
			binary.Write(&tmp, f.ByteOrder(), uint32(sh.Offset))
			binary.Write(&tmp, f.ByteOrder(), uint32(sh.Size))
			binary.Write(&tmp, f.ByteOrder(), uint32(sh.Link))
			binary.Write(&tmp, f.ByteOrder(), uint32(sh.Info))
			binary.Write(&tmp, f.ByteOrder(), uint32(sh.AddrAlign))
			binary.Write(&tmp, f.ByteOrder(), uint32(sh.EntSize))
		} else {
			binary.Write(&tmp, f.ByteOrder(), sh.Flags)
			binary.Write(&tmp, f.ByteOrder(), sh.Addr)
			binary.Write(&tmp, f.ByteOrder(), sh.Offset)
			binary.Write(&tmp, f.ByteOrder(), sh.Size)
			binary.Write(&tmp, f.ByteOrder(), sh.Link)
			binary.Write(&tmp, f.ByteOrder(), sh.Info)
			binary.Write(&tmp, f.ByteOrder(), sh.AddrAlign)
			binary.Write(&tmp, f.ByteOrder(), sh.EntSize)
		}
	}
	return &tmp, nil
}

func writeProgramHeaders(f *File) (io.Reader, error) {
	var tmp bytes.Buffer
	for _, ph := range f.Programs {
		binary.Write(&tmp, f.ByteOrder(), ph.Type)
		if f.Is32() {
			binary.Write(&tmp, f.ByteOrder(), uint32(ph.Offset))
			binary.Write(&tmp, f.ByteOrder(), uint32(ph.VirtualAddr))
			binary.Write(&tmp, f.ByteOrder(), uint32(ph.PhysicalAddr))
			binary.Write(&tmp, f.ByteOrder(), uint32(ph.FileSize))
			binary.Write(&tmp, f.ByteOrder(), uint32(ph.MemSize))
			binary.Write(&tmp, f.ByteOrder(), uint32(ph.Flags))
			binary.Write(&tmp, f.ByteOrder(), uint32(ph.Alignment))
		} else {
			binary.Write(&tmp, f.ByteOrder(), ph.Flags)
			binary.Write(&tmp, f.ByteOrder(), ph.Offset)
			binary.Write(&tmp, f.ByteOrder(), ph.VirtualAddr)
			binary.Write(&tmp, f.ByteOrder(), ph.PhysicalAddr)
			binary.Write(&tmp, f.ByteOrder(), ph.FileSize)
			binary.Write(&tmp, f.ByteOrder(), ph.MemSize)
			binary.Write(&tmp, f.ByteOrder(), ph.Alignment)
		}
	}
	return &tmp, nil
}

func writeHeader(f *File) (io.Reader, error) {
	var tmp bytes.Buffer

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
	binary.Write(&tmp, f.ByteOrder(), f.ShCount)
	binary.Write(&tmp, f.ByteOrder(), f.NamesIndex)

	return &tmp, nil
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
	for i := range list {
		linkIx := list[i].Link
		if linkIx == 0 {
			continue
		}
		linkName := f.Sections[linkIx].Label
		ix := slices.IndexFunc(list, func(sh SectionHeader) bool {
			return sh.Label == linkName
		})
		if ix >= 0 {
			list[i].Link = uint32(ix)
		}
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
	Type         uint32
	Flags        uint32
	Offset       uint64
	VirtualAddr  uint64
	PhysicalAddr uint64
	FileSize     uint64
	MemSize      uint64
	Alignment    uint64
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

func (s SectionHeader) isLoadable() bool {
	return s.Type == 0x1 || s.Type == 0x5
}

func (s SectionHeader) isDynamic() bool {
	return s.Type == 0x6
}

func (s SectionHeader) isTLS() bool {
	return s.Flags&0x400 == 0x400
}

func (s SectionHeader) isNote() bool {
	return s.Type == 0x7
}

func (s SectionHeader) canStrip() bool {
	if strings.HasPrefix(s.Label, ".debug") {
		return true
	}
	// if strings.HasPrefix(s.Label, ".note") {
	// 	return true
	// }
	return s.Label == ".comment" || s.Label == ".symtab" || s.Label == ".strtab"
}

func getProgramTypeName(ph ProgramHeader) string {
	switch ph.Type {
	case 0x0:
		return "NULL"
	case 0x1:
		return "LOAD"
	case 0x2:
		return "DYNAMIC"
	case 0x3:
		return "INTERP"
	case 0x4:
		return "NOTE"
	case 0x5:
		return "RESERVED"
	case 0x6:
		return "SELF"
	case 0x7:
		return "TLS"
	case 0x6474e551:
		return "GNU-PTR"
	default:
		return "OTHER"
	}
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
		return "OTHER"
	}
}

func setLabels(elf *ELFHeader, r io.ReaderAt) error {
	var (
		ns  = elf.Sections[elf.NamesIndex]
		buf = make([]byte, ns.Size)
		rs  = io.NewSectionReader(r, int64(ns.Offset), int64(ns.Size))
	)
	if _, err := io.ReadFull(rs, buf); err != nil {
		return err
	}
	for i, sh := range elf.Sections {
		if uint64(sh.Name) >= ns.Size {
			continue
		}
		ix := bytes.IndexByte(buf[sh.Name:], 0)
		if ix < 0 {
			continue
		}
		elf.Sections[i].Label = string(buf[sh.Name : int(sh.Name)+ix])
	}
	return nil
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
	return elf, setLabels(elf, r)
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
		ph.FileSize = uint64(sizeFile)
		ph.MemSize = uint64(sizeMem)
		ph.Alignment = uint64(align)
		ph.Flags = flags
	} else {
		binary.Read(r, elf.ByteOrder(), &ph.Flags)
		binary.Read(r, elf.ByteOrder(), &ph.Offset)
		binary.Read(r, elf.ByteOrder(), &ph.VirtualAddr)
		binary.Read(r, elf.ByteOrder(), &ph.PhysicalAddr)
		binary.Read(r, elf.ByteOrder(), &ph.FileSize)
		binary.Read(r, elf.ByteOrder(), &ph.MemSize)
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
