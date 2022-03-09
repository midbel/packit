package deb

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/packit"
)

func ParseControl(r io.Reader) (packit.Metadata, error) {
	var meta packit.Metadata
	return meta, parseControl(r, &meta)
}

func parseControl(r io.Reader, meta *packit.Metadata) error {
	rs := bufio.NewReader(r)
	for {
		field, value, err := tryField(rs)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		switch field {
		default:
			return fmt.Errorf("%s: unsupported/unknown field", field)
		case "Package":
			meta.Package = value
		case "Version":
			meta.Version = value
		case "Essential":
			meta.Essential = true
		case "Section":
			meta.Section = value
		case "Priority":
			meta.Priority = value
		case "Date":
			dt, err := time.Parse(debDateFormat, value)
			if err != nil {
				return err
			}
			meta.Date = dt
		case "Architecture":
			switch value {
			case debArch64:
				meta.Arch = packit.Arch64
			case debArch32:
				meta.Arch = packit.Arch32
			case debArchAll:
			default:
				return fmt.Errorf("%s: invalid architecture value", value)
			}
		case "Vendor":
			meta.Vendor = value
		case "Maintainer":
			addr, err := mail.ParseAddress(value)
			meta.Maintainer.Name = value
			if err == nil {
				meta.Maintainer.Name = addr.Name
				meta.Maintainer.Email = addr.Address
			}
		case "Homepage":
			meta.Home = value
		case "Depends":
		case "Suggests":
		case "Provides":
		case "Conflicts":
		case "Replaces":
		case "Build-Using":
			meta.Compiler = value
		case "Installed-Size":
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			meta.Size = i
		case "Description":
			meta.Summary = value
			meta.Desc = strings.Join(tryContinuation(rs), "\n")
		}
	}
	return nil
}

func tryField(rs *bufio.Reader) (string, string, error) {
	line, err := rs.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(line) == "" {
		return "", "", io.EOF
	}
	line = strings.TrimSpace(line)
	field, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", fmt.Errorf("invalid line: %q", line)
	}
	return field, strings.TrimSpace(value), nil
}

func tryContinuation(rs *bufio.Reader) []string {
	var list []string
	for {
		bs, err := rs.Peek(1)
		if err != nil {
			break
		}
		if len(bs) >= 1 && bs[0] != ' ' && bs[0] != '\t' {
			break
		}
		line, err := rs.ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "" {
			break
		}
		list = append(list, strings.TrimSpace(line))
	}
	return list
}
