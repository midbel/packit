package rpm

import (
	"os"
	"io"
)

func Check(file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	return nil
}

func readLead(r io.Reader) error {
	return nil
}

func readSignatures(r io.Reader) error {
	return nil
}

func readSums(r io.Reader) error {
	return nil
}

func checkFiles(r io.Reader) error {
	return nil
}