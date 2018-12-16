package packit

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/midbel/tape"
	"github.com/midbel/tape/cpio"
)

type RPM struct {
	*Makefile
}

func (r *RPM) PackageName() string {
	return r.Control.PackageName() + ".rpm"
}

func (r *RPM) Build(w io.Writer) error {
	if err := r.writeLead(w); err != nil {
		return err
	}
	var data, meta bytes.Buffer
	if err := r.writeData(&data); err != nil {
		return err
	}
	if err := r.writeHeader(&meta); err != nil {
		return err
	}
	return nil
}

func (r *RPM) writeData(w io.Writer) error {
	wc := cpio.NewWriter(w)

	digest := md5.New()
	for _, i := range r.Files {
		f, err := os.Open(i.Src)
		if err != nil {
			return err
		}
		var (
			size int64
			r    io.Reader
		)
		if i.Compress {
			var body bytes.Buffer
			z := gzip.NewWriter(&body)
			if _, err := io.Copy(z, f); err != nil {
				return err
			}
			if err := z.Close(); err != nil {
				return err
			}
			r, size = &body, int64(body.Len())
		} else {
			s, err := f.Stat()
			if err != nil {
				return err
			}
			size, r = s.Size(), f
		}
		h := tape.Header{
			Filename: i.String(),
			Mode:     int64(i.Mode()),
			Length:   size,
			Uid:      0,
			Gid:      0,
			ModTime:  time.Now().Truncate(time.Minute),
		}
		if err := wc.WriteHeader(&h); err != nil {
			return err
		}
		if i.Size, err = io.Copy(io.MultiWriter(wc, digest), r); err != nil {
			return err
		}
		i.Sum = fmt.Sprintf("%x", digest.Sum(nil))

		f.Close()
		digest.Reset()
	}
	return wc.Close()
}

func (r *RPM) writeHeader(w io.Writer) error {
	return nil
}

func (r *RPM) writeSignatures(w io.Writer) error {
	return nil
}

func (r *RPM) writeLead(w io.Writer) error {
	return nil
}
