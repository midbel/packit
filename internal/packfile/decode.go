package packfile

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/midbel/packit/internal/glob"
)

//go:embed licenses/*
var licenseFiles embed.FS

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
	optDistrib         = "distrib"
	optVendor          = "vendor"
	optUrl             = "url"
	optHome            = "home"
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

var errSkip = errors.New("skip")

const maxIncluded = 255

func defaultChecker(err error) error {
	return err
}

type DecoderConfig struct {
	NoIgnore   bool
	DryRun     bool
	IgnoreFile string
	Packfile   string
	Type       string
	Licenses   string

	EnvFile string
}

func (d DecoderConfig) getMatcher() (glob.Matcher, error) {
	if d.NoIgnore {
		return glob.Default(), nil
	}
	r, err := os.Open(d.IgnoreFile)
	if err != nil && d.IgnoreFile != "" {
		return nil, err
	}
	defer r.Close()

	return glob.Parse(r)
}

type Decoder struct {
	context string
	file    string
	nested  int
	parent  *Decoder

	ignore       glob.Matcher
	errorChecker func(error) error

	scan         *Scanner
	curr         Token
	peek         Token
	templateMode bool

	licenses *template.Template

	env *Environ
}

func NewDecoder(context string, config *DecoderConfig) (*Decoder, error) {
	if context == "" {
		return nil, fmt.Errorf("context missing")
	}
	r, err := os.Open(config.Packfile)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	d := createDecoder(r, context, defaultEnv())
	d.file = r.Name()
	d.ignore, err = config.getMatcher()
	if err != nil {
		return nil, err
	}

	if tpl := template.New("license"); config.Licenses != "" {
		d.licenses, err = tpl.ParseGlob(config.Licenses)
	} else {
		d.licenses, err = tpl.ParseFS(licenseFiles, "licenses/*.tpl")
	}
	if err != nil {
		return nil, err
	}
	d.next()
	d.next()

	return d, nil
}

func createDecoder(r io.Reader, context string, env *Environ) *Decoder {
	d := Decoder{
		context:      context,
		errorChecker: defaultChecker,
		ignore:       glob.Default(),
		scan:         Scan(r),
		env:          Enclosed(env),
	}

	return &d
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
	return &pkg, d.DecodeInto(&pkg)
}

func (d *Decoder) DecodeInto(pkg *Package) error {
	return d.decode(pkg)
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
				Flags:   FileFlagDoc,
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
				Flags:   FileFlagDoc,
			}
			pkg.Files = append(pkg.Files, res)
		default:
			err = fmt.Errorf("license: %s unsupported option", option)
		}
		return err
	}, false)
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
		Flags:   FileFlagDoc,
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
	}, false)
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
	}, true)
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
	}, false)
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
	var (
		all []Resource
		res Resource
	)

	d.errorChecker = func(err error) error {
		if errors.Is(err, glob.ErrIgnore) {
			return nil
		}
		return err
	}
	defer func() {
		d.errorChecker = defaultChecker
	}()

	err := d.decodeObject(func(option string) error {
		var err error
		switch option {
		case optFileSource:
			path, err1 := d.decodeString()
			if err1 != nil {
				return err1
			}
			paths, err := glob.Walk(path, d.context)
			if err != nil {
				return err
			}
			for _, p := range slices.Collect(paths) {
				if err := d.ignore.Match(p); err != nil {
					continue
				}
				r := Resource{
					Path: p,
				}
				all = append(all, r)
			}
		case optFileGhost:
			ok, err := d.decodeBool()
			if err != nil {
				return err
			}
			if ok {
				res.Flags |= FileFlagGhost
			}
		case optFileDoc:
			ok, err := d.decodeBool()
			if err != nil {
				return err
			}
			if ok {
				res.Flags |= FileFlagDoc
			}
		case optFileConf, optFileConfig:
			ok, err := d.decodeBool()
			if err != nil {
				return err
			}
			if ok {
				res.Flags |= FileFlagConf
			}
		case optFileLicense:
			ok, err := d.decodeBool()
			if err != nil {
				return err
			}
			if ok {
				res.Flags |= FileFlagLicense
			}
		case optFileReadme:
			ok, err := d.decodeBool()
			if err != nil {
				return err
			}
			if ok {
				res.Flags |= FileFlagReadme
			}
		case optFileTarget:
			res.Target, err = d.decodeString()
			if err == nil {
				res.Target = filepath.Clean(res.Target)
				res.Target = filepath.ToSlash(res.Target)
			}
		case optFilePerm:
			perm, err1 := d.decodeString()
			if err1 != nil {
				return err1
			}
			res.Perm, err = strconv.ParseInt(perm, 0, 64)
		case optFileCompress:
			res.Compress, err = d.decodeBool()
		default:
			err = fmt.Errorf("file: %s unsupported option", option)
		}
		return err
	}, false)
	if err != nil {
		return err
	}
	for _, r := range all {
		r.Local, err = d.openFile(r.Path)
		if err != nil {
			return err
		}
		r.Target = res.Target
		if len(all) > 1 {
			res.Target = filepath.Join(res.Target, filepath.Base(r.Path))
		}
		r.Perm = res.Perm
		r.Compress = res.Compress
		r.Flags = res.Flags

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
	return nil
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
	}, false)
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
	case optDistrib:
		pkg.Distrib, err = d.decodeString()
	case optVendor:
		pkg.Vendor, err = d.decodeString()
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
	case optHome, optUrl:
		pkg.Home, err = d.decodeString()
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

func (d *Decoder) decodeObject(do func(option string) error, allowDuplicates bool) error {
	if !d.is(BegObj) {
		return fmt.Errorf("object: missing opening brace")
	}
	d.next()

	d.env = Enclosed(d.env)
	defer func() {
		d.env = d.env.unwrap()
	}()

	skip := func() {
		for !d.done() && !d.is(EndObj) {
			d.next()
		}
	}

	seen := make(map[string]struct{})
	for !d.done() && !d.is(EndObj) {
		if !d.is(Literal) {
			return fmt.Errorf("object property must be literal string")
		}
		option := d.getCurrentLiteral()
		if _, ok := seen[option]; ok && !allowDuplicates {
			return fmt.Errorf("object: duplicate option %s", option)
		}
		seen[option] = struct{}{}
		d.next()
		if err := do(option); err != nil {
			err = d.errorChecker(err)
			if errors.Is(err, errSkip) {
				skip()
				break
			}
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
		err = d.executeInclude(pkg)
	case "let":
		err = d.executeLet()
	case "env":
		err = d.executeEnv()
	case "echo":
		err = d.executeEcho()
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
	switch macro {
	case "readfile":
		err = d.executeReadFile()
	case "exec":
		err = d.executeExec()
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

func (d *Decoder) executeEcho() error {
	var parts []string
	for !d.isEOL() && !d.done() {
		parts = append(parts, d.getCurrentLiteral())
		d.next()
	}
	fmt.Fprintln(os.Stdout, strings.Join(parts, " "))
	if !d.isEOL() {
		return fmt.Errorf("missing eol after echo")
	}
	d.skipEOL()
	return nil
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

func (d *Decoder) executeInclude(pkg *Package) error {
	file, err := d.decodeString()
	if err != nil {
		return err
	}
	r, err := os.Open(filepath.Join(d.context, file))
	if err != nil {
		return err
	}
	defer r.Close()

	sub := createDecoder(r, d.context, d.env)
	sub.nested = d.nested + 1
	sub.parent = d
	if sub.nested > maxIncluded {
		return fmt.Errorf("too many level of included files")
	}
	if sub.isIncluded() {
		return fmt.Errorf("file already included")
	}
	sub.file = r.Name()
	sub.licenses = d.licenses
	sub.next()
	sub.next()
	return sub.DecodeInto(pkg)
}

func (d *Decoder) isIncluded() bool {
	curr := d.parent
	for {
		if curr == nil {
			break
		}
		if curr.file == d.file {
			return true
		}
		curr = curr.parent
	}
	return false
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
	if d.done() {
		return
	}
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
