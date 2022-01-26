package packit

import (
	"time"
)

type Metadata struct {
	Name    string
	Version string

	Resources []Resource
}

type Resource struct {
	File string
	Perm int
}

type Change struct {
	Title string
	Desc  string
	When  time.Time
}
