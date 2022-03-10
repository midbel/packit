package apk

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/packit"
	"github.com/midbel/packit/text"
	"github.com/midbel/tape/tar"
)

const (
	apkControlFile   = "control.tar.gz"
	apkDataFile      = "data.tar.gz"
	apkPackageFile   = ".PKGINFO"
	apkSignatureFile = ".SIGN"
)

func Extract(file, dir string, flat, all bool) error {
	return nil
}

func Info(file string) (packit.Metadata, error) {
	return packit.Metadata{}, nil
}

func Verify(file string) error {
	return nil
}

func List(file string) ([]packit.Resource, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return nil, nil
}

func Build(dir string, meta packit.Metadata) error {
	w, err := os.Create(filepath.Join(dir, getPackageName(meta)))
	if err != nil {
		return err
	}
	defer w.Close()
	return build(w, meta)
}

func build(w io.Writer, meta packit.Metadata) error {
	var (
		data bytes.Buffer
		hash = sha256.New()
	)
	if err := writeData(io.MultiWriter(&data, hash), meta); err != nil {
		return err
	}
	meta.DataHash = fmt.Sprintf("%x", hash.Sum(nil))
	if err := writeControl(w, meta); err != nil {
		return err
	}
	_, err := io.Copy(w, &data)
	return err
}

func writeControl(w io.Writer, meta packit.Metadata) error {
	var (
		tmp bytes.Buffer
		buf = gzip.NewWriter(&tmp)
		tw  = tar.NewWriter(buf)
	)
	buf.Header.Name = apkControlFile
	if err := appendControlFile(tw, meta); err != nil {
		return err
	}
	tw.Flush()
	buf.Close()

	_, err := io.Copy(w, &tmp)
	return err
}

func writeData(w io.Writer, meta packit.Metadata) error {
	sort.Slice(meta.Resources, func(i, j int) bool {
		return meta.Resources[i].Path() < meta.Resources[j].Path()
	})
	var (
		tmp  bytes.Buffer
		buf  = gzip.NewWriter(&tmp)
		tw   = tar.NewWriter(buf)
		dirs = make(map[string]struct{})
	)
	buf.Header.Name = apkDataFile
	for _, r := range meta.Resources {
		if err := appendResource(tw, r, dirs); err != nil {
			return err
		}
	}
	tw.Close()
	buf.Close()
	_, err := io.Copy(w, &tmp)
	return err
}

func appendResource(tw *tar.Writer, res packit.Resource, dirs map[string]struct{}) error {
	var (
		dir = filepath.Dir(res.Path())
		tmp string
	)
	for _, d := range strings.Split(dir, "/") {
		tmp = filepath.Join(tmp, d)
		if _, ok := dirs[tmp]; ok {
			continue
		}
		dirs[tmp] = struct{}{}
		h := getTarHeaderDir(tmp, res.ModTime)
		if err := tw.WriteHeader(&h); err != nil {
			return err
		}
	}
	r, err := os.Open(res.File)
	if err != nil {
		return err
	}
	defer r.Close()

	var (
		buf bytes.Buffer
		w   io.Writer = &buf
	)
	if res.Compress {
		w, _ = gzip.NewWriterLevel(w, gzip.BestCompression)
	}
	if res.Size, err = io.Copy(w, r); err != nil {
		return err
	}
	if c, ok := w.(io.Closer); ok {
		c.Close()
	}
	h := getTarHeaderFile(res.Path(), buf.Len(), res.ModTime)
	h.PaxHeaders["APK-TOOLS.checksum.SHA1"] = res.Digest
	if err := tw.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(tw, &buf)
	return err
}

//go:embed control.tpl
var controlfile string

var fmap = template.FuncMap{}

func appendControlFile(tw *tar.Writer, meta packit.Metadata) error {
	tpl, err := template.New("control").Funcs(fmap).Parse(controlfile)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := text.Execute(tpl, &buf, meta); err != nil {
		return err
	}

	h := getTarHeaderFile(apkPackageFile, buf.Len(), meta.Date)
	if err := tw.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(tw, &buf)
	return err
}

func getPackageName(meta packit.Metadata) string {
	return fmt.Sprintf("%s-%s.%s", meta.Package, meta.Version, packit.APK)
}

func getTarHeaderFile(file string, size int, when time.Time) tar.Header {
	return tar.Header{
		Name:    file,
		Perm:    0644,
		Size:    int64(size),
		ModTime: when,
		Gid:     0,
		Uid:     0,
		Type:    tar.TypeReg,
		PaxHeaders: map[string]string{
			"atime": "0",
			"mtime": "0",
		},
	}
}

func getTarHeaderDir(file string, when time.Time) tar.Header {
	return tar.Header{
		Name:    file,
		Perm:    0755,
		ModTime: when,
		Gid:     0,
		Uid:     0,
		Type:    tar.TypeDir,
		PaxHeaders: map[string]string{
			"atime": "0",
			"mtime": "0",
		},
	}
}
