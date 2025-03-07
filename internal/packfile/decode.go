package packfile

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

//go:embed licenses/*
var licenseFiles embed.FS

type Environ struct {
	parent *Environ
	values map[string]any
}

func Empty() *Environ {
	e := Environ{
		values: make(map[string]any),
	}
	return &e
}

func (e *Environ) Define(ident string, value any) error {
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

const (
	optFile            = "file"
	optFileSource      = "source"
	optFileGhost       = "ghost"
	optFileDoc         = "doc"
	optFileConf        = "conf"
	optFileLicense     = "license"
	optFileReadme      = "readme"
	optFileTarget      = "target"
	optFilePerm        = "perm"
	optFileCompress    = "compress"
	optFileConfig      = "config"
	optLicense         = "license"
	optCopyright       = "copyright"
	optLicenseText     = "text"
	optLicenseType     = "type"
	optLicenseFile     = "file"
	optMaintainer      = "maintainer"
	optMaintainerName  = "name"
	optMaintainerEmail = "email"
	optChange          = "changelog"
	optChangeSummary   = "summary"
	optChangeChange    = "change"
	optChangeVersion   = "version"
	optChangeDate      = "date"
	optDepends         = "depends"
	optDependsPackage  = "package"
	optDependsType     = "type"
	optDependsArch     = "arch"
	optDependsVersion  = "version"
	optCompiler        = "compiler"
	optCompilerName    = "name"
	optCompilerVersion = "version"
	optSetup           = "setup"
	optTeardown        = "teardown"
	optPackage         = "package"
	optName            = "name"
	optSummary         = "summary"
	optRelease         = "release"
	optVersion         = "version"
	optDesc            = "desc"
	optDescLong        = "description"
	optPriority        = "priority"
	optSection         = "section"
	optGroup           = "group"
	optType            = "type"
	optOs              = "os"
	optArch            = "arch"
	optArchLong        = "architecture"
	optPreInst         = "pre-install"
	optPreRem          = "pre-remove"
	optPostInst        = "post-install"
	optPostRem         = "post-remove"
	optCheckPkg        = "check-package"
)

type Decoder struct {
	context string

	macros map[string]func() (string, error)

	scan         *Scanner
	curr         Token
	peek         Token
	templateMode bool

	licenses *template.Template

	env *Environ
}

func NewDecoder(r io.Reader, context string) (*Decoder, error) {
	d := Decoder{
		context: context,
		scan:    Scan(r),
		env:     Empty(),
		macros:  make(map[string]func() (string, error)),
	}

	licenses, err := template.New("license").ParseFS(licenseFiles, "licenses/*.tpl")
	if err != nil {
		return nil, err
	}
	d.licenses = licenses
	d.next()
	d.next()

	return &d, nil
}

func (d *Decoder) Decode() (*Package, error) {
	pkg := Package{
		Version:  DefaultVersion,
		Os:       DefaultOS,
		Section:  DefaultSection,
		Priority: DefaultPriority,
		License:  DefaultLicense,
		Arch:     ArchNo,
	}
	return &pkg, d.decode(&pkg)
}

func (d *Decoder) RegisterMacro(macro string, do func() (string, error)) {
	d.macros[macro] = do
}

func (d *Decoder) decode(pkg *Package) error {
	for !d.done() {
		d.skipComment()

		var err error
		switch {
		case d.is(Literal):
			err = d.decodeOption(pkg)
		case d.is(Macro):
			err = d.decodeMainMacro(pkg)
		default:
			err = fmt.Errorf("syntax error: identifier or macro expected")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) decodeLicense(pkg *Package) error {
	if d.is(String) || d.is(Literal) {
		return d.decodeLicenseFromTemplate(pkg)
	}
	return d.decodeLicenseFromObject(pkg)
}

func (d *Decoder) decodeLicenseFromObject(pkg *Package) error {
	return d.decodeObject(func(option string) error {
		var err error
		switch option {
		case optLicenseType:
			pkg.License, err = d.decodeString()
		case optLicenseText:
			text, err := d.decodeString()
			if err != nil {
				return err
			}
			res := Resource{
				Local:   io.NopCloser(strings.NewReader(text)),
				Target:  filepath.Join(DirDoc, pkg.Name, copyrightFile),
				Perm:    PermFile,
				Size:    int64(len(text)),
				Lastmod: time.Now(),
			}
			pkg.Files = append(pkg.Files, res)
		case optLicenseFile:
			file, err := d.decodeString()
			if err != nil {
				return err
			}
			r, err := os.Open(file)
			if err != nil {
				return err
			}
			s, err := r.Stat()
			if err != nil {
				return err
			}
			res := Resource{
				Local:   r,
				Target:  filepath.Join(DirDoc, pkg.Name, copyrightFile),
				Perm:    PermFile,
				Size:    s.Size(),
				Lastmod: s.ModTime(),
			}
			pkg.Files = append(pkg.Files, res)
		default:
			err = fmt.Errorf("license: %s unsupported option", option)
		}
		return err
	})
}

func (d *Decoder) decodeLicenseFromTemplate(pkg *Package) error {
	license, err := d.decodeString()
	if err != nil {
		return err
	}
	ctx := struct {
		Date    time.Time
		Author  string
		Package string
	}{
		Date:    time.Now(),
		Author:  pkg.Maintainer.Name,
		Package: pkg.Name,
	}
	var str bytes.Buffer
	if err := d.licenses.ExecuteTemplate(&str, license+".tpl", ctx); err != nil {
		return err
	}
	pkg.License = strings.ToUpper(license)

	file := Resource{
		Local:   io.NopCloser(&str),
		Target:  filepath.Join(DirDoc, pkg.Name, copyrightFile),
		Perm:    PermFile,
		Size:    int64(str.Len()),
		Lastmod: time.Now(),
	}
	pkg.Files = append(pkg.Files, file)
	return nil
}

func (d *Decoder) decodeScript() (string, error) {
	script, err := d.decodeString()
	if err != nil {
		return "", err
	}
	buf, err := os.ReadFile(script)
	if err == nil {
		return string(buf), nil
	}
	return script, nil
}

func (d *Decoder) decodePackageMaintainer(pkg *Package) error {
	m, err := d.decodeMaintainer()
	if err == nil {
		pkg.Maintainer = m
	}
	return err
}

func (d *Decoder) decodeMaintainer() (Maintainer, error) {
	var m Maintainer

	return m, d.decodeObject(func(option string) error {
		var err error
		switch option {
		case optMaintainerName:
			m.Name, err = d.decodeString()
		case optMaintainerEmail:
			m.Email, err = d.decodeString()
		default:
			err = fmt.Errorf("maintainer: %s unsupported option", option)
		}
		return err
	})
}

func (d *Decoder) decodeChange(pkg *Package) error {
	var c Change

	err := d.decodeObject(func(option string) error {
		var err error
		switch option {
		case optChangeSummary:
			c.Summary, err = d.decodeString()
		case optChangeChange:
			line, err1 := d.decodeString()
			if err1 != nil {
				return err1
			}
			c.Changes = append(c.Changes, line)
		case optChangeVersion:
			c.Version, err = d.decodeString()
		case optChangeDate:
			c.When, err = d.decodeDate()
		case optMaintainer:
			c.Maintainer, err = d.decodeMaintainer()
		default:
			err = fmt.Errorf("change: %s unsupported option", option)
		}
		return err
	})
	if err == nil {
		pkg.Changes = append(pkg.Changes, c)
	}
	return err
}

func (d *Decoder) decodeDepends(pkg *Package) error {
	var p Dependency

	err := d.decodeObject(func(option string) error {
		var err error
		switch option {
		case optDependsPackage:
			p.Package, err = d.decodeString()
		case optDependsType:
			p.Type, err = d.decodeString()
		case optDependsArch:
			p.Arch, err = d.decodeString()
		case optDependsVersion:
			p.Constraint = ConstraintEq
			p.Version = d.getCurrentLiteral()
			d.next()
			if d.is(String) {
				p.Constraint = p.Version
				p.Version = d.getCurrentLiteral()
				d.next()
			}
			if !d.isEOL() {
				return fmt.Errorf("missing end of line after value")
			}
			d.skipEOL()
		default:
			err = fmt.Errorf("dependency: %s unsupported option", option)
		}
		return err
	})
	if err == nil {
		pkg.Depends = append(pkg.Depends, p)
	}
	return err
}

func (d *Decoder) openLocalFile(file string) (io.ReadCloser, error) {
	return os.Open(file)
}

func (d *Decoder) openRemoteFile(file string) (io.ReadCloser, error) {
	res, err := http.Get(file)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote file can not be retrieved")
	}
	defer res.Body.Close()

	w, err := os.CreateTemp("", "pack.*.dat")
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(w, res.Body); err != nil {
		return nil, err
	}
	w.Seek(0, os.SEEK_SET)
	return w, nil
}

func (d *Decoder) openFile(file string) (io.ReadCloser, error) {
	u, err := url.Parse(file)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http", "https":
		return d.openRemoteFile(file)
	default:
		return d.openLocalFile(filepath.Join(d.context, file))
	}
}

func (d *Decoder) decodeFile(pkg *Package) error {
	var r Resource

	err := d.decodeObject(func(option string) error {
		var err error
		switch option {
		case optFileSource:
			path, err1 := d.decodeString()
			if err1 != nil {
				return err1
			}
			r.Local, err = d.openFile(path)
		case optFileGhost:
			ok, err := d.decodeBool()
			if err != nil {
				return err
			}
			if ok {
				r.Flags |= FileFlagGhost
			}
		case optFileDoc:
			ok, err := d.decodeBool()
			if err != nil {
				return err
			}
			if ok {
				r.Flags |= FileFlagDoc
			}
		case optFileConf, optFileConfig:
			ok, err := d.decodeBool()
			if err != nil {
				return err
			}
			if ok {
				r.Flags |= FileFlagConf
			}
		case optFileLicense:
			ok, err := d.decodeBool()
			if err != nil {
				return err
			}
			if ok {
				r.Flags |= FileFlagLicense
			}
		case optFileReadme:
			ok, err := d.decodeBool()
			if err != nil {
				return err
			}
			if ok {
				r.Flags |= FileFlagReadme
			}
		case optFileTarget:
			r.Target, err = d.decodeString()
			if err == nil {
				r.Target = strings.ReplaceAll(r.Target, "\\", "/")
			}
		case optFilePerm:
			perm, err1 := d.decodeString()
			if err1 != nil {
				return err1
			}
			r.Perm, err = strconv.ParseInt(perm, 0, 64)
		case optFileCompress:
			r.Compress, err = d.decodeBool()
		default:
			err = fmt.Errorf("file: %s unsupported option", option)
		}
		return err
	})
	if err == nil {
		file := r.Local.(*os.File)
		s, err := file.Stat()
		if err != nil {
			return err
		}
		if !s.Mode().IsRegular() {
			return fmt.Errorf("regular file expected")
		}
		r.Size = s.Size()
		r.Lastmod = s.ModTime()
		if r.Perm == 0 {
			r.Perm = GetPermissionFromPath(r.Target)
		}
		pkg.Files = append(pkg.Files, r)
	}
	return err
}

func (d *Decoder) decodeCompiler(pkg *Package) error {
	if d.is(String) || d.is(Literal) {
		return d.decodeCompilerFromString(pkg)
	}
	return d.decodeCompilerFromObject(pkg)
}

func (d *Decoder) decodeCompilerFromString(pkg *Package) error {
	pkg.BuildWith.Name = d.getCurrentLiteral()
	d.next()
	if !d.is(String) && !d.is(Literal) && !d.is(Number) {
		return fmt.Errorf("compiler: missing version")
	}
	pkg.BuildWith.Version = d.getCurrentLiteral()
	d.next()
	if !d.isEOL() {
		return fmt.Errorf("missing end of line after value")
	}
	d.skipEOL()
	return nil
}

func (d *Decoder) decodeCompilerFromObject(pkg *Package) error {
	return d.decodeObject(func(option string) error {
		var err error
		switch option {
		case optCompilerName:
			pkg.BuildWith.Name, err = d.decodeString()
		case optCompilerVersion:
			pkg.BuildWith.Version, err = d.decodeString()
		default:
			return fmt.Errorf("compiler: %s unsupported option", option)
		}
		return err
	})
	return nil
}

func (d *Decoder) decodeOption(pkg *Package) error {
	var (
		option = d.getCurrentLiteral()
		err    error
	)
	d.next()
	switch option {
	case optSetup:
		pkg.Setup, err = d.decodeString()
	case optTeardown:
		pkg.Teardown, err = d.decodeString()
	case optPackage, optName:
		pkg.Name, err = d.decodeString()
	case optRelease:
		pkg.Release, err = d.decodeString()
	case optSummary:
		pkg.Summary, err = d.decodeString()
	case optDesc, optDescLong:
		pkg.Desc, err = d.decodeString()
	case optVersion:
		pkg.Version, err = d.decodeString()
	case optPriority:
		pkg.Priority, err = d.decodeString()
	case optSection, optGroup:
		pkg.Section, err = d.decodeString()
	case optLicense, optCopyright:
		err = d.decodeLicense(pkg)
	case optCompiler:
		err = d.decodeCompiler(pkg)
	case optType:
		pkg.PackageType, err = d.decodeString()
	case optOs:
		pkg.Os, err = d.decodeString()
	case optArch, optArchLong:
		pkg.Arch, err = d.decodeString()
	case optMaintainer:
		err = d.decodePackageMaintainer(pkg)
	case optFile:
		err = d.decodeFile(pkg)
	case optChange:
		err = d.decodeChange(pkg)
	case optDepends:
		err = d.decodeDepends(pkg)
	case optPreInst:
		pkg.PreInst, err = d.decodeString()
	case optPostInst:
		pkg.PostInst, err = d.decodeString()
	case optPreRem:
		pkg.PreRem, err = d.decodeString()
	case optPostRem:
		pkg.PostRem, err = d.decodeString()
	case optCheckPkg:
	default:
		err = fmt.Errorf("package %s unsupported option", option)
	}
	return err
}

func (d *Decoder) decodeBool() (bool, error) {
	if !d.is(Boolean) {
		return false, fmt.Errorf("value can not be used as a boolean")
	}
	ok, err := strconv.ParseBool(d.getCurrentLiteral())
	if err != nil {
		return false, err
	}
	d.next()
	if !d.isEOL() {
		return false, fmt.Errorf("eol expected after bool value")
	}
	d.skipEOL()
	return ok, nil
}

func (d *Decoder) decodeString() (string, error) {
	var (
		str string
		err error
	)
	switch {
	case d.is(Literal) || d.is(String) || d.is(Number) || d.is(Heredoc):
		str = d.getCurrentLiteral()
		d.next()
	case d.is(Macro):
		str, err = d.decodeMacro()
	default:
		err = fmt.Errorf("value can not be used as a string")
	}
	if err != nil {
		return "", err
	}
	if !d.isEOL() {
		return "", fmt.Errorf("eol expected after string value")
	}
	d.skipEOL()
	return str, nil
}

func (d *Decoder) decodeDate() (time.Time, error) {
	str, err := d.decodeString()
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse("2006-01-02", str)
}

func (d *Decoder) decodeObject(do func(option string) error) error {
	if !d.is(BegObj) {
		return fmt.Errorf("object: missing opening brace")
	}
	d.next()

	// seen := make(map[string]struct{})
	for !d.done() && !d.is(EndObj) {
		if !d.is(Literal) {
			return fmt.Errorf("object property must be literal string")
		}
		option := d.getCurrentLiteral()
		// if _, ok := seen[option]; ok {
		// 	return fmt.Errorf("object: duplicate option %s", option)
		// }
		// seen[option] = struct{}{}
		d.next()
		if err := do(option); err != nil {
			return err
		}
	}
	if !d.is(EndObj) {
		return fmt.Errorf("object: missing closing brace")
	}
	d.next()
	if !d.isEOL() {
		return fmt.Errorf("eol expected after object")
	}
	d.skipEOL()
	return nil
}

func (d *Decoder) decodeMainMacro(pkg *Package) error {
	var (
		macro = d.getCurrentLiteral()
		err   error
	)
	d.next()
	switch macro {
	case "include":
		err = d.executeInclude()
	case "let":
		err = d.executeLet()
	case "env":
		err = d.executeEnv()
	case "define":
	case "apply":
	default:
		err = fmt.Errorf("%s is not a supported macro", macro)
	}
	return err
}

func (d *Decoder) decodeMacro() (string, error) {
	var (
		macro = d.getCurrentLiteral()
		err   error
		str   string
	)
	d.next()
	if fn, ok := d.macros[macro]; ok {
		return fn()
	}
	switch macro {
	case "readfile":
		err = d.executeReadFile()
	case "exec":
		err = d.executeExec()
	case "arch64":
		str = Arch64
	case "arch32":
		str = Arch32
	case "noarch":
		str = ArchNo
	case "all":
		str = ArchAll
	case "etcdir":
		str = DirEtc
	case "vardir":
		str = DirVar
	case "logdir":
		str = DirLog
	case "optdir":
		str = DirOpt
	case "bindir":
		str = DirBin
	case "usrbindir":
		str = DirBinUsr
	case "docdir":
		str = DirDoc
	default:
		err = fmt.Errorf("%s is not a supported macro", macro)
	}
	return str, err
}

func (d *Decoder) executeEnv() error {
	if !d.is(Literal) {
		return fmt.Errorf("variable name should be valid identifier")
	}
	ident := d.getCurrentLiteral()
	d.next()

	value, err := d.decodeString()
	if err == nil {
		err = os.Setenv(ident, value)
	}
	return err
}

func (d *Decoder) executeLet() error {
	if !d.is(Literal) {
		return fmt.Errorf("variable name should be valid identifier")
	}
	ident := d.getCurrentLiteral()
	d.next()

	value, err := d.decodeString()
	if err == nil {
		err = d.env.Define(ident, value)
	}
	return err
}

func (d *Decoder) executeExec() error {
	args := []string{
		"-c",
		d.getCurrentLiteral(),
	}
	cmd := exec.Command(DefaultShell, args...)
	buf, err := cmd.Output()
	if err != nil {
		d.curr.Type = Invalid
		return err
	}
	d.curr.Literal = string(buf)
	d.curr.Type = String
	return nil
}

func (d *Decoder) executeReadFile() error {
	buf, err := os.ReadFile(d.getCurrentLiteral())
	if err == nil {
		d.curr.Type = String
		d.curr.Literal = string(buf)
	} else {
		d.curr.Type = Invalid
	}
	return nil
}

func (d *Decoder) executeInclude() error {
	file, err := d.decodeString()
	if err != nil {
		return err
	}
	r, err := os.Open(filepath.Join(d.context, file))
	if err != nil {
		return err
	}
	defer r.Close()
	return nil
}

func (d *Decoder) getCurrentLiteral() string {
	return d.curr.Literal
}

func (d *Decoder) isEOL() bool {
	return d.is(EOL) || d.is(Comment) || d.is(EOF)
}

func (d *Decoder) is(kind rune) bool {
	return d.curr.Type == kind
}

func (d *Decoder) skipEOL() {
	for d.isEOL() {
		d.next()
		if d.done() {
			break
		}
	}
}

func (d *Decoder) skipComment() {
	for d.is(Comment) {
		d.next()
	}
}

func (d *Decoder) next() {
	d.curr = d.peek
	d.peek = d.scan.Scan()

	switch {
	case d.is(Template) && !d.templateMode:
		d.replaceTemplate()
	case d.is(LocalVar):
		d.replaceLocal()
	case d.is(EnvVar):
		d.replaceEnv()
	default:
	}
}

func (d *Decoder) replaceLocal() {
	str, err := d.env.Resolve(d.getCurrentLiteral())
	if err != nil {
		d.curr.Type = Invalid
	} else {
		d.curr.Type = String
		d.curr.Literal, _ = str.(string)
	}
}

func (d *Decoder) replaceEnv() {
	str := os.Getenv(d.getCurrentLiteral())
	d.curr.Literal = str
	d.curr.Type = String
}

func (d *Decoder) replaceTemplate() {
	d.enterTemplate()
	defer d.leaveTemplate()
	d.next()
	var list []string
	for !d.done() && !d.is(Template) {
		list = append(list, d.getCurrentLiteral())
		d.next()
	}
	if !d.is(Template) {
		d.curr.Type = Invalid
	} else {
		d.curr.Type = String
		d.curr.Literal = strings.Join(list, "")
	}
}

func (d *Decoder) done() bool {
	return d.is(EOF)
}

func (d *Decoder) enterTemplate() {
	d.templateMode = true
}

func (d *Decoder) leaveTemplate() {
	d.templateMode = false
}
