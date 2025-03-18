package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/midbel/tape/ar"
)

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
