package packit

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"os"
	"time"

	"github.com/midbel/tape"
	"github.com/midbel/tape/cpio"
)

const (
	rpmMagic = 0xedabeedb
	rpmMajor = 3
	rpmMinor = 0
	rpmBinary = 0
	rpmSigType = 5
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

	md5sum := md5.New()
	sha256sum := sha256.New()
	sha512sum := sha512.New()
	var body bytes.Buffer

	ws := io.MultiWriter(&body, md5sum, sha256sum, sha512sum)
	if _, err := io.Copy(ws, io.MultiReader(&meta, &data)); err != nil {
		return err
	}
	if err := r.writeSums(w, body.Len(), md5sum, sha256sum, sha512sum); err != nil {
		return err
	}
	_, err := io.Copy(w, &body)
	return err
}

func (r *RPM) writeSums(w io.Writer, n int, md, sh256, sh512 hash.Hash) error {
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
	body := make([]byte, 96)
	binary.BigEndian.PutUint32(body[0:], uint32(rpmMagic))
	binary.BigEndian.PutUint16(body[4:], uint16(rpmMajor)<<8 | uint16(rpmMinor))
	binary.BigEndian.PutUint16(body[6:], rpmBinary)
	binary.BigEndian.PutUint16(body[8:], 0)
	if n := []byte(r.PackageName()); len(n) <= 65 {
		copy(body[10:], n)
	} else {
		copy(body[10:], n[:65])
	}
	binary.BigEndian.PutUint16(body[76:], 0)
	binary.BigEndian.PutUint16(body[78:], rpmSigType)

	_, err := w.Write(body)
	return err
}
