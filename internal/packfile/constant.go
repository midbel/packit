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
	ArchNo  = "noarch"
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

const copyrightFile = "copyright"

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
	DefaultUser     = "root"
	DefaultGroup    = "root"
)

const (
	ConstraintEq = "eq"
	ConstraintNe = "ne"
	ConstraintGt = "gt"
	ConstraintGe = "ge"
	ConstraintLt = "lt"
	ConstraintLe = "le"
)

const (
	FileFlagConf         = 1 << 0
	FileFlagDoc          = 1 << 1
	FileFlagAllowMissing = 1 << 3
	FileFlagNoReplace    = 1 << 4
	FileFlagGhost        = 1 << 6
	FileFlagLicense      = 1 << 7
	FileFlagReadme       = 1 << 8
	FileFlagExec         = 1 << 9
	FileFlagDir          = 1 << 12

	FileFlagRegular = FileFlagConf | FileFlagDoc | FileFlagLicense | FileFlagReadme
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
