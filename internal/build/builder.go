package build

import (
	"github.com/midbel/packit/internal/packfile"
)

type Builder interface {
	Build(*packfile.Package) error
}
