package packit

func ArchString(a uint8) string {
	switch a {
	case Arch32:
		return "i386"
	case Arch64:
		return "x86_64"
	default:
		return "noarch"
	}
}
