package rpm

import (
	"compress/gzip"
	"crypto/md5"
	"io"

	"github.com/midbel/mack"
	"github.com/midbel/mack/cpio"
)

const MagicRPM = 0xedabeedb
const MagicHDR = 0x008eade8

const (
	MajorRPM = 3
	MinorRPM = 0
)

type builder struct {
	inner io.Writer

	md5sums   []string
	filenames []string
	size      int64
}

func NewBuilder(w io.Writer) mack.Builder {
	return &builder{w}
}

func (w *builder) Build(c mack.Control, files []*mack.File) error {
	body, err := w.writeArchive(files)
	if err != nil {
		return err
	}
	return nil
}

func (w *build) writeArchive(files []*mack.File) (*bytes.Buffer, error) {
	body := new(bytes.Buffer)
	ark := cpio.NewWriter(body)
	for _, f := range files {
		bs, err := writeFile(ark, f)
		if err != nil {
			return nil, err
		}
		w.md5sums = append(w.md5sums, fmt.Sprintf("%x", bs))
		w.filenames = append(w.filenames, f.String())
	}
	if err := ark.Close(); err != nil {
		return nil, err
	}
	bz := new(bytes.Buffer)
	w, _ := gzip.NewWriterLevel(bz, gzip.BestCompression)
	if _, err := io.Copy(w, body); err != nil {
		return nil, err
	}
	return bz, nil
}

func writeFile(w *cpio.Writer, f *mack.File) ([]byte, error) {
	r, err := os.Open(f.Src)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	i, err := r.Stat()
	if err != nil {
		return nil, err
	}
	stat, ok := i.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return nil, fmt.Errorf("can not get stat for info %s", f)
	}
	h := cpio.Header{
		Filename: r.String(),
		Mode:     int64(i.Mode()),
		Length:   i.Size(),
		ModTime:  i.ModTime(),
		Major:    int64(stat.Dev >> 32),
		Minor:    int64(stat.Dev & 0xFFFFFFFF),
	}
	if err := w.WriteHeader(&h); err != nil {
		return nil, err
	}
	s := md5.New()
	if _, err := io.Copy(io.MultiWriter(w, s), r); err != nil {
		return nil, err
	}
	return s.Sum(nil), err
}
