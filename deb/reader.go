package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/midbel/mack"
	"github.com/midbel/tape/ar"
)

type File struct {
	Name    string
	Size    int64
	Mode    uint32
	ModTime time.Time
}

type Package struct {
	io.Closer

	control *bytes.Reader
	data    *bytes.Reader
}

func Open(file string) (*Package, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	// md5sum := md5.New()
	// sha1sum := sha1.New()
	// sha2sum := sha256.New()
	// sha5sum := sha512.New()
	//
	// w := io.MultiWriter(md5sum, sha1sum, sha2sum, sha5sum)
	// a, err := ar.NewReader(io.TeeReader(r, w))
	a, err := ar.NewReader(r)
	if err != nil {
		return nil, err
	}

	p := Package{Closer: r}
	for {
		h, err := a.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			r.Close()
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		switch h.Filename {
		case DebDataTar:
			r, err := readBytes(a, h.Length)
			if err != nil {
				return nil, err
			}
			p.data = r
		case DebControlTar:
			r, err := readBytes(a, h.Length)
			if err != nil {
				return nil, err
			}
			p.control = r
		case DebBinaryFile:
			if err := isSupported(a); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown filename %s", h.Filename)
		}
	}
	return &p, nil
}

func (p *Package) Control() (*mack.Control, error) {
	if _, err := p.control.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(p.control)
	if err != nil {
		return nil, err
	}
	r := tar.NewReader(z)
	for {
		h, err := r.Next()
		if err != nil {
			return nil, err
		}
		if h.Name == DebControlFile {
			return readControl(r)
		}
	}
}

func (p *Package) Check() (bool, error) {
	return false, nil
}

func (p *Package) Files() ([]File, error) {
	return listFiles(p.data)
}

func listFiles(r io.ReadSeeker) ([]File, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer z.Close()

	t := tar.NewReader(z)

	var fs []File
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		f := File{
			Name:    h.Name,
			Size:    h.Size,
			ModTime: h.ModTime,
		}
		fs = append(fs, f)
	}
	return fs, nil
}

func readControl(r io.Reader) (*mack.Control, error) {
	s := scan(r)
	var ctrl mack.Control
	for k, v, err := s.Scan(); err == nil; k, v, err = s.Scan() {
		switch strings.ToLower(k) {
		case "package":
			ctrl.Package = v
		case "version":
			ctrl.Version = v
		case "license":
			ctrl.License = v
		case "section":
			ctrl.Section = v
		case "priority":
			ctrl.Priority = v
		case "archictecture":
			ctrl.Arch = v
		case "vendor":
			ctrl.Vendor = v
		case "homepage":
			ctrl.Home = v
		case "pre-depends":
			ctrl.Depends = strings.Split(v, ", ")
		case "build-using":
			ctrl.Compiler = v
		case "description":
			lines := strings.SplitN(v, "\n", 2)
			ctrl.Summary, ctrl.Desc = lines[0], lines[1]
		}
	}
	return &ctrl, nil
}

func readBytes(r io.Reader, n int64) (*bytes.Reader, error) {
	bs := make([]byte, int(n))
	if _, err := io.ReadFull(r, bs); err != nil {
		return nil, err
	}
	return bytes.NewReader(bs), nil
}

func isSupported(r io.Reader) error {
	vs := []byte(DebVersion)

	var w bytes.Buffer
	if _, err := io.CopyN(&w, r, int64(len(vs))); err != nil {
		return err
	}

	if !bytes.Equal(vs, w.Bytes()) {
		return fmt.Errorf("unsupported deb version: %s", w.String())
	}
	return nil
}
