package env

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func Export(r io.Reader) error {
	values, err := Load(r)
	if err != nil {
		return err
	}
	for n, v := range values {
		if err := os.Setenv(n, v); err != nil {
			return err
		}
	}
	return nil
}

func Load(r io.Reader) (map[string]string, error) {
	return parse(r)
}

func parse(r io.Reader) (map[string]string, error) {
	var (
		list = make(map[string]string)
		scan = bufio.NewScanner(r)
	)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		key, value, err := parseLine(line)
		if err != nil {
			return nil, err
		}
	}
	return list, nil
}

func parseLine(line string) (string, string, error) {
	k, v, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", fmt.Errorf("value should be separated from key by equal sign")
	}
	if err := parseValue(v); err != nil {
		return "", "", err
	}
	return k, v, nil
}

func parseValue(value string) error {
	return nil
}
