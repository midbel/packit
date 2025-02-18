package packfile

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/template"
	"time"
)

type Decoder struct {
	scan         *Scanner
	curr         Token
	peek         Token
	templateMode bool

	licenses *template.Template

	env *Environ
}

func NewDecoder(r io.Reader) (*Decoder, error) {
	d := Decoder{
		scan: Scan(r),
		env:  Empty(),
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
		Arch:     Arch64,
	}
	return &pkg, d.decode(&pkg)
}

func (d *Decoder) decode(pkg *Package) error {
	for !d.done() {
		d.skipComment()

		var err error
		switch {
		case d.is(Literal):
			err = d.decodeOption(pkg)
		case d.is(Macro):
			err = d.decodeMacro(pkg)
		default:
			err = fmt.Errorf("syntax error: identifier or macro expected")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) decodeLicenseFromTemplate(pkg *Package, name string) error {
	ctx := struct {
		Date    time.Time
		Author  string
		Package string
	}{
		Date:    time.Now(),
		Author:  pkg.Maintainers[0].Name,
		Package: pkg.Name,
	}
	var str strings.Builder
	if err := d.licenses.ExecuteTemplate(&str, name+".tpl", ctx); err != nil {
		return err
	}
	pkg.License = str.String()
	return nil
}

func (d *Decoder) decodeLicense(pkg *Package) error {
	var license string
	license, err := d.decodeString()
	if err != nil {
		return err
	}
	ok := strings.Contains(d.licenses.DefinedTemplates(), license)
	if ok {
		return d.decodeLicenseFromTemplate(pkg, license)
	}
	buf, err := os.ReadFile(license)
	if err == nil {
		license = string(buf)
	}
	pkg.License = license
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
		pkg.Maintainers = append(pkg.Maintainers, m)
		return err
	}
	return err
}

func (d *Decoder) decodeMaintainer() (Maintainer, error) {
	var m Maintainer

	return m, d.decodeObject(func(option string) error {
		var err error
		switch option {
		case "name":
			m.Name, err = d.decodeString()
		case "email":
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
		case "summary":
			c.Summary, err = d.decodeString()
		case "description":
			c.Desc, err = d.decodeString()
		case "version":
			c.Version, err = d.decodeString()
		case "date":
			c.When, err = d.decodeDate()
		case "maintainer":
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
		case "package":
			p.Package, err = d.decodeString()
		case "type":
			p.Type, err = d.decodeString()
		case "arch":
			p.Arch, err = d.decodeString()
		case "version":
			p.Constraint = constraintEq
			p.Version = d.getCurrentLiteral()
			d.next()
			if d.is(String) {
				p.Constraint, err = getVersionContraint(p.Version)
				if err != nil {
					return err
				}
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

func (d *Decoder) decodeFile(pkg *Package) error {
	var r Resource

	err := d.decodeObject(func(option string) error {
		var err error
		switch option {
		case "source":
			path, err1 := d.decodeString()
			if err1 != nil {
				return err1
			}
			r.Local, err = os.Open(path)
		case "target":
			r.Target, err = d.decodeString()
		case "perm":
			perm, err1 := d.decodeString()
			if err1 != nil {
				return err1
			}
			r.Perm, err = strconv.ParseInt(perm, 0, 64)
		case "compress":
			_, err = d.decodeBool()
		default:
			err = fmt.Errorf("file: %s unsupported option", option)
		}
		return err
	})
	if err == nil {
		s, err := r.Local.Stat()
		if err != nil {
			return err
		}
		if !s.Mode().IsRegular() {
			return fmt.Errorf("regular file expected")
		}
		r.size = s.Size()
		r.lastmod = s.ModTime()
		if r.Perm == 0 {
			r.Perm = int64(s.Mode().Perm())
		}
		pkg.Files = append(pkg.Files, r)
	}
	return err
}

func (d *Decoder) decodeOption(pkg *Package) error {
	var (
		option = d.getCurrentLiteral()
		err    error
	)
	d.next()
	switch option {
	case "package", "name":
		pkg.Name, err = d.decodeString()
	case "summary":
		pkg.Summary, err = d.decodeString()
	case "description":
		pkg.Desc, err = d.decodeString()
	case "version":
		pkg.Version, err = d.decodeString()
	case "priority":
		pkg.Priority, err = d.decodeString()
	case "section":
		pkg.Section, err = d.decodeString()
	case "license":
		err = d.decodeLicense(pkg)
	case "compiler":
		pkg.Compiler, err = d.decodeString()
	case "type":
		pkg.PackageType, err = d.decodeString()
	case "os":
		pkg.Os, err = d.decodeString()
	case "arch", "architecture":
		pkg.Arch, err = d.decodeString()
	case "maintainer":
		err = d.decodePackageMaintainer(pkg)
	case "file":
		err = d.decodeFile(pkg)
	case "change":
		err = d.decodeChange(pkg)
	case "depends":
		err = d.decodeDepends(pkg)
	case "pre-install":
		pkg.PreInst, err = d.decodeString()
	case "post-install":
		pkg.PostInst, err = d.decodeString()
	case "pre-remove":
		pkg.PreRem, err = d.decodeString()
	case "post-remove":
		pkg.PostRem, err = d.decodeString()
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
	if !d.isEOL() {
		return false, fmt.Errorf("eolf expected after bool value")
	}
	d.skipEOL()
	return ok, nil
}

func (d *Decoder) decodeString() (string, error) {
	var str string
	switch {
	case d.is(Literal) || d.is(String) || d.is(Number) || d.is(Heredoc):
		str = d.getCurrentLiteral()
		d.next()
	case d.is(Macro):
	default:
		return "", fmt.Errorf("value can not be used as a string")
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

	seen := make(map[string]struct{})
	for !d.done() && !d.is(EndObj) {
		if !d.is(Literal) {
			return fmt.Errorf("object property must be literal string")
		}
		option := d.getCurrentLiteral()
		if _, ok := seen[option]; ok {
			return fmt.Errorf("object: duplicate option %s", option)
		}
		seen[option] = struct{}{}
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

func (d *Decoder) decodeMacro(pkg *Package) error {
	var (
		macro = d.getCurrentLiteral()
		err   error
	)
	d.next()
	switch macro {
	case "include":
		err = d.executeInclude()
	case "readfile":
		err = d.executeReadFile()
	case "exec":
		err = d.executeExec()
	case "define":
	case "apply":
	case "let":
		err = d.executeLet()
	default:
		err = fmt.Errorf("%s is not a supported macro", macro)
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
