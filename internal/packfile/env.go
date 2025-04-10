package packfile

import "fmt"

type Environ struct {
	parent   *Environ
	values   map[string]any
	readonly bool
}

func Empty() *Environ {
	return Enclosed(nil)
}

func Enclosed(parent *Environ) *Environ {
	env := Environ{
		parent: parent,
		values: make(map[string]any),
	}
	return &env
}

func defaultEnv() *Environ {
	env := Empty()

	env.Define("arch64", Arch64)
	env.Define("arch32", Arch32)
	env.Define("noarch", ArchNo)
	env.Define("archall", ArchAll)
	env.Define("etcdir", DirEtc)
	env.Define("vardir", DirVar)
	env.Define("logdir", DirLog)
	env.Define("optdir", DirOpt)
	env.Define("bindir", DirBin)
	env.Define("usrbindir", DirBinUsr)
	env.Define("docdir", DirDoc)

	env.readonly = true
	return env
}

func (e *Environ) Define(ident string, value any) error {
	if e.readonly {
		return fmt.Errorf("%s can not be modified as it is readonly")
	}
	_, ok := e.values[ident]
	if ok {
		return fmt.Errorf("identifier %q already defined", ident)
	}
	e.values[ident] = value
	return nil
}

func (e *Environ) Resolve(ident string) (any, error) {
	v, ok := e.values[ident]
	if ok {
		return v, nil
	}
	if e.parent != nil {
		return e.parent.Resolve(ident)
	}
	return nil, fmt.Errorf("undefined variable %s", ident)
}

func (e *Environ) unwrap() *Environ {
	if e.parent == nil {
		return e
	}
	return e.parent
}
