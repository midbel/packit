package deb

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/midbel/packit/internal/packfile"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
)

func Content(file string) ([]*tape.Header, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	rs, err := ar.NewReader(r)
	if err != nil {
		return nil, err
	}
	if err := readDebian(rs); err != nil {
		return nil, err
	}
	h, err := rs.Next()
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(io.Discard, io.LimitReader(rs, h.Size)); err != nil {
		return nil, err
	}
	dt, err := openFile(rs, DataFile)
	if err != nil {
		return nil, err
	}
	var list []*tape.Header
	for {
		h, err := dt.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if _, err := io.Copy(io.Discard, io.LimitReader(dt, h.Size)); err != nil {
			return nil, err
		}
		hdr := tape.Header{
			Filename: h.Name,
			Size:     h.Size,
			Mode:     h.Mode,
			Uid:      int64(h.Uid),
			Gid:      int64(h.Gid),
			ModTime:  h.ModTime,
		}
		if h.Typeflag == tar.TypeDir {
			hdr.Mode |= int64(os.ModeDir)
		}
		list = append(list, &hdr)
	}
	return list, nil
}

type PackageInfo struct {
	packfile.Package
	Size int64
}

func Info(file string) (*PackageInfo, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	rs, err := ar.NewReader(r)
	if err != nil {
		return nil, err
	}
	if err := readDebian(rs); err != nil {
		return nil, err
	}
	control, err := readControl(rs, controlFile)
	if err != nil {
		return nil, err
	}
	return parseControl(control)
}

func Dependencies(file string) ([]string, error) {
	return nil, nil
}

func parseControl(r io.Reader) (*PackageInfo, error) {
	var (
		scan = bufio.NewScanner(r)
		pkg  PackageInfo
	)
	for scan.Scan() {
		line := scan.Text()
		if line == "" {
			break
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid control file: missing colon in line %s", line)
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(field) {
		case "package":
			pkg.Name = value
		case "version":
			pkg.Version = value
		case "maintainer":
		case "section":
			pkg.Section = value
		case "priority":
			pkg.Priority = value
		case "architecture":
			pkg.Arch = value
		case "built-using":
			pkg.BuildWith.Name = value
		case "installed-size":
			size, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			pkg.Size = size
		case "description":
			pkg.Summary = value
			var lines []string
			for scan.Scan() {
				line := strings.TrimSpace(scan.Text())
				if line == "" {
					break
				}
				if line == "." {
					line = ""
				}
				lines = append(lines, strings.TrimSpace(line))
			}
			lines = append(lines, "")
			pkg.Desc = strings.Join(lines, "\n")
		default:
		}
	}
	return &pkg, nil
}
