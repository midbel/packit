package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/midbel/cli"
	"github.com/midbel/tape/ar"
)

func runShow(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	f, err := os.Open(cmd.Flag.Arg(0))
	if err != nil {
		return err
	}
	rs := bufio.NewReader(f)
	bs, err := rs.Peek(16)
	if err != nil {
		return err
	}
	var show func(io.Reader) error
	switch {
	case bytes.HasPrefix(bs, ar.Magic):
		show = showDEB
	default:
		return fmt.Errorf("unknown packet type")
	}
	return show(rs)
}

func showDEB(r io.Reader) error {
	return fmt.Errorf("not yet implemented")
}

func showRPM(r io.Reader) error {
	return fmt.Errorf("not yet implemented")
}
