package rpm

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/midbel/tape/cpio"
)

func Check(file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := readLead(r); err != nil {
		return err
	}

	sig, err := readSignatures(r)
	if err != nil {
		return err
	}
	var (
		sh1 = sha1.New()
		sh2 = sha256.New()
		md  = md5.New()
		sum = io.MultiWriter(sh1, sh2, md)
	)
	_, err = readSums(io.TeeReader(r, sum))
	if err != nil {
		return err
	}
	sum = io.MultiWriter(sh1, md)
	if err := checkFiles(io.TeeReader(r, sum), nil); err != nil {
		return err
	}
	_ = sig
	return nil
}

type rpmSignature struct {
	TotalLen    int64
	DataLen     int64
	HeaderHash  string
	DataMD5Hash string
	DataSHAHash string
}

type rpmFileDigest struct {
	File   string
	Digest string
}

func readSignatures(r io.Reader) (*rpmSignature, error) {
	var (
		buf   = make([]byte, len(rpmHeader))
		count int32
		size  int32
	)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	if !bytes.Equal(buf, rpmHeader) {
		return nil, fmt.Errorf("signature: not a valid rpm header (%x)", buf)
	}
	binary.Read(r, binary.BigEndian, &count)
	binary.Read(r, binary.BigEndian, &size)

	var (
		index bytes.Buffer
		tmp   bytes.Buffer
	)
	if _, err := io.CopyN(&index, r, int64(rpmEntryLen*count)); err != nil {
		return nil, err
	}
	if _, err := io.CopyN(&tmp, r, int64(size)); err != nil {
		return nil, err
	}
	var (
		sig   rpmSignature
		store = bytes.NewReader(tmp.Bytes())
	)
	for i := 0; i < int(count); i++ {
		var (
			tag    int32
			kind   int32
			offset int32
			size   int32
		)
		binary.Read(&index, binary.BigEndian, &tag)
		binary.Read(&index, binary.BigEndian, &kind)
		binary.Read(&index, binary.BigEndian, &offset)
		binary.Read(&index, binary.BigEndian, &size)

		_, err := store.Seek(int64(offset), io.SeekStart)
		if err != nil {
			return nil, err
		}
		switch rs := bufio.NewReader(store); tag {
		case rpmSigSha1:
			sig.HeaderHash, err = rs.ReadString(0)
			if err != nil {
				return nil, err
			}
		case rpmSigSha256:
			sig.DataSHAHash, err = rs.ReadString(0)
			if err != nil {
				return nil, err
			}
		case rpmSigMD5:
			buf := make([]byte, int(size))
			if _, err = io.ReadFull(rs, buf); err != nil {
				return nil, err
			}
		case rpmSigLength:
			binary.Read(store, binary.BigEndian, &size)
			sig.DataLen = int64(size)
		case rpmSigPayload:
			binary.Read(store, binary.BigEndian, &size)
			sig.TotalLen = int64(size)
		default:
		}
	}
	sigLength := tmp.Len() + index.Len() + rpmEntryLen
	if mod := sigLength % 8; mod != 0 {
		zs := make([]byte, 8-mod)
		io.ReadFull(r, zs)
	}
	return &sig, nil
}

func readSums(r io.Reader) ([]rpmFileDigest, error) {
	var (
		buf   = make([]byte, len(rpmHeader))
		count int32
		size  int32
	)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	if !bytes.Equal(buf, rpmHeader) {
		return nil, fmt.Errorf("header: not a valid rpm header (%x)", buf)
	}
	binary.Read(r, binary.BigEndian, &count)
	binary.Read(r, binary.BigEndian, &size)

	var (
		index bytes.Buffer
		store bytes.Buffer
	)
	if _, err := io.CopyN(&index, r, int64(rpmEntryLen*count)); err != nil {
		return nil, err
	}
	if _, err := io.CopyN(&store, r, int64(size)); err != nil {
		return nil, err
	}
	return nil, nil
}

func checkFiles(r io.Reader, digests []rpmFileDigest) error {
	z, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	cp := cpio.NewReader(z)
	for {
		h, err := cp.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		io.CopyN(io.Discard, cp, int64(h.Size))
	}
	return nil
}

func readLead(r io.Reader) error {
	lead := make([]byte, rpmLeadLen)
	if _, err := io.ReadFull(r, lead); err != nil {
		return err
	}
	if !bytes.HasPrefix(lead, rpmMagic) {
		return fmt.Errorf("not a rpm file - invalid magic (%x)", lead[:len(rpmMagic)])
	}
	return nil
}
