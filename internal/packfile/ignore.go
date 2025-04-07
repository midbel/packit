package packfile

import (
	"errors"
)

var ErrIgnore = errors.New("ignore")

type Matcher interface {
	Match(string) error
}

type all struct{}

func (_ all) Match(_ string) error {
	return nil
}

func keepAll() Matcher {
	return all{}
}

type matcherSet struct {
	patterns []Matcher
}

func Open(file string) (Matcher, error) {
	return nil, nil
}

func (m matcherSet) Match(file string) error {
	for i := range m.patterns {
		err := m.patterns[i].Match(file)
		if err == nil {
			return nil
		}
	}
	return ErrIgnore
}
