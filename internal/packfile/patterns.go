package packfile

import (
	"errors"
)

var ErrIgnore = errors.New("ignore")

type Accepter interface {
	Accept(string) bool
}

type all struct{}

func (_ all) Accept(_ string) bool {
	return true
}

func acceptAll() Accepter {
	return all{}
}
