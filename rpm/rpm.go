package rpm

const Magic = 0xedabeedb

type Lead struct {
	Magic     int32
	Major     int8
	Minor     int8
	Type      int16
	Arch      int16
	Os        int16
	Signature int16
	Name      [66]byte
}

func (l *Lead) String() {
	return string(l.Name)
}
