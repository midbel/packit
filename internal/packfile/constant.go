package packfile

import (
	"path/filepath"
	"strings"
)

const (
	Deb = "deb"
	Rpm = "rpm"
	Apk = "apk"
)

const (
	Arch64  = "amd64"
	Arch32  = "i386"
	ArchAll = "all"
)

const (
	PermFile = 0o644
	PermExec = 0o755
	PermDir  = 0o755
)

const (
	DirEtc    = "etc"
	DirVar    = "var"
	DirLog    = "var/log"
	DirOpt    = "opt"
	DirBin    = "bin"
	DirBinUsr = "usr/bin"
	DirDoc    = "usr/share/doc"
)

const (
	EnvMaintainerName = "PACK_MAINTAINER_NAME"
	EnvMaintainerMail = "PACK_MAINTAINER_MAIL"
)

const (
	EnvArchive = "archive"
	EnvBash    = "bash"
	EnvShell   = "shell"

	envHash = "hash"
)

const (
	DefaultVersion  = "0.1.0"
	DefaultLicense  = "mit"
	DefaultSection  = "utils"
	DefaultPriority = "optional"
	DefaultOS       = "linux"
	DefaultShell    = "/bin/sh"
)

const (
	ConstraintEq = "eq"
	ConstraintNe = "ne"
	ConstraintGt = "gt"
	ConstraintGe = "ge"
	ConstraintLt = "lt"
	ConstraintLe = "le"
)

func GetPermissionFromPath(file string) int64 {
	dir := filepath.Dir(file)
	if strings.Contains(dir, DirBin) {
		return PermExec
	} else if strings.Contains(dir, DirEtc) {
		return PermFile
	} else if strings.Contains(dir, DirDoc) {
		return PermFile
	}
	return PermFile
}
