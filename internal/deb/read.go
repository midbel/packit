package deb

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/midbel/tape/ar"
)

func Check(file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	rs, err := ar.NewReader(r)
	if err != nil {
		return err
	}
	if err := readDebian(rs); err != nil {
		return err
	}
	conf, err := readControl(rs, md5File)
	if err != nil {
		return err
	}
	sums, err := readChecksums(conf)
	if err != nil {
		return err
	}
	return checkFiles(rs, sums)
}

func checkFiles(r *ar.Reader, sums map[string]string) error {
	rs, err := openFile(r, DataFile)
	if err != nil {
		return err
	}
	for {
		h, err := rs.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		value, ok := sums[h.Name]
		if !ok {
			return fmt.Errorf("%s: file in %s but not in %s", h.Name, DataFile, md5File)
		}
		md := md5.New()
		if _, err := io.Copy(md, io.LimitReader(rs, h.Size)); err != nil {
			return err
		}
		if hex.EncodeToString(md.Sum(nil)) != value {
			return fmt.Errorf("%s: checksum mismatched (%s)", h.Name, value)
		}
	}
	return nil
}

func readChecksums(r io.Reader) (map[string]string, error) {
	var (
		scan = bufio.NewScanner(r)
		list = make(map[string]string)
	)
	for scan.Scan() {
		line := scan.Text()
		if line == "" {
			break
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("%s: line badly formatted (%s)", md5File, line)
		}
		list[parts[1]] = parts[0]
	}
	return list, nil
}

func readDebian(r *ar.Reader) error {
	h, err := r.Next()
	if err != nil {
		return err
	}
	if h.Filename != debianFile {
		return fmt.Errorf("%s expected but got %s", debianFile, h.Filename)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(r, h.Size)); err != nil {
		return err
	}
	if buf.String() != debVersion {
		return fmt.Errorf("invalid debian version: got %s but expected %s", buf.String(), debVersion)
	}
	return nil
}

func readControl(r *ar.Reader, file string) (io.Reader, error) {
	rs, err := openFile(r, ControlFile)
	if err != nil {
		return nil, err
	}
	for {
		h, err := rs.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if h.Name == file {
			var tmp bytes.Buffer
			if _, err := io.Copy(&tmp, io.LimitReader(rs, h.Size)); err != nil {
				return nil, err
			}
			return &tmp, nil
		}
		if _, err := io.Copy(io.Discard, io.LimitReader(rs, h.Size)); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("file %s not found in %s", file, ControlFile)
}

func openFile(r *ar.Reader, file string) (*tar.Reader, error) {
	h, err := r.Next()
	if err != nil {
		return nil, err
	}
	if h.Filename != file {
		return nil, fmt.Errorf("%s expected but got %s", file, h.Filename)
	}
	z, err := gzip.NewReader(io.LimitReader(r, h.Size))
	if err != nil {
		return nil, err
	}
	return tar.NewReader(z), nil
}
