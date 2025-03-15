package rpm

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"os"

	"github.com/midbel/packit/internal/packfile"
)

func Info(file string) (*packfile.Package, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if err := readLead(r); err != nil {
		return nil, err
	}
	if err := readHeader(r, io.Discard, io.Discard, true); err != nil {
		return nil, err
	}
	var (
		index bytes.Buffer
		store bytes.Buffer
	)
	if err := readHeader(r, &index, &store, false); err != nil {
		return nil, err
	}
	return readPackage(&index, bytes.NewReader(store.Bytes()), index.Len()/16)
}

func readPackage(index io.Reader, store io.ReadSeeker, total int) (*packfile.Package, error) {
	var pkg packfile.Package

	for i := 0; i < total; i++ {
		var (
			tag    int32
			kind   int32
			offset int32
			count  int32
		)
		binary.Read(index, binary.BigEndian, &tag)
		binary.Read(index, binary.BigEndian, &kind)
		binary.Read(index, binary.BigEndian, &offset)
		binary.Read(index, binary.BigEndian, &count)

		store.Seek(int64(offset), io.SeekStart)

		var err error
		switch tag {
		case rpmTagPackage:
			pkg.Name, err = readString(store)
		case rpmTagVersion:
			pkg.Version, err = readString(store)
		case rpmTagRelease:
			pkg.Release, err = readString(store)
		case rpmTagSummary:
			pkg.Summary, err = readString(store)
		case rpmTagDesc:
			pkg.Desc, err = readString(store)
		case rpmTagDistrib:
			pkg.Distrib, err = readString(store)
		case rpmTagVendor:
			pkg.Vendor, err = readString(store)
		case rpmTagLicense:
			pkg.License, err = readString(store)
		case rpmTagPackager:
			pkg.Maintainer.Name, err = readString(store)
		case rpmTagGroup:
			pkg.Section, err = readString(store)
		case rpmTagURL:
			pkg.Home, err = readString(store)
		case rpmTagArch:
			pkg.Arch, err = readString(store)
		default:
		}
		if err != nil {
			return nil, err
		}
	}

	return &pkg, nil
}

func readString(r io.Reader) (string, error) {
	tmp := bufio.NewReader(r)
	return tmp.ReadString(0)
}
