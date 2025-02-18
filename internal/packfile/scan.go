package packfile

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"iter"
	"unicode/utf8"
)

type Position struct {
	Line   int
	Column int
}

const (
	EOF rune = -(1 << iota)
	EOL
	Comment
	LocalVar
	EnvVar
	Literal
	String
	Heredoc
	Template
	Number
	Boolean
	BegObj
	EndObj
	BegArr
	EndArr
	BegGrp
	EndGrp
	Comma
	Macro
	Invalid
)

var types = map[rune]string{
	EOF:      "eof",
	Literal:  "literal",
	LocalVar: "local-var",
	EnvVar:   "env-var",
	Comment:  "comment",
	Heredoc:  "heredoc",
	String:   "string",
	Template: "template",
	Number:   "number",
	Boolean:  "boolean",
	BegArr:   "beg-arr",
	EndArr:   "end-arr",
	BegObj:   "beg-obj",
	EndObj:   "end-obj",
	BegGrp:   "beg-grp",
	EndGrp:   "end-grp",
	Comma:    "comma",
	EOL:      "eol",
	Macro:    "macro",
	Invalid:  "invalid",
}

type Token struct {
	Literal string
	Type    rune
	Position
}

func (t Token) String() string {
	prefix, ok := types[t.Type]
	if !ok {
		return "<unknown>"
	}
	if t.Literal == "" {
		return fmt.Sprintf("<%s>", prefix)
	}
	return fmt.Sprintf("<%s(%s)>", prefix, t.Literal)
}

type Scanner struct {
	input io.RuneScanner
	char  rune
	str   bytes.Buffer

	Position
	old Position

	templateMode bool
}

func Scan(r io.Reader) *Scanner {
	scan := &Scanner{
		input: bufio.NewReader(r),
	}
	scan.Position.Line = 1
	scan.read()
	return scan
}

func (s *Scanner) All() iter.Seq[Token] {
	fn := func(yield func(Token) bool) {
		for {
			tok := s.Scan()
			if !yield(tok) || tok.Type == EOF {
				break
			}
		}
	}
	return fn
}

func (s *Scanner) Scan() Token {
	defer s.reset()
	var tok Token
	tok.Position = s.Position
	if s.done() {
		tok.Type = EOF
		return tok
	}
	if s.templateMode {
		s.scanTemplate(&tok)
		return tok
	}
	if isNL(s.char) {
		s.skipBlank()
		tok.Type = EOL
		return tok
	}
	s.skipBlank()
	switch {
	case isDigit(s.char):
		s.scanNumber(&tok)
	case isQuote(s.char):
		s.scanQuote(&tok)
	case isMacro(s.char):
		s.scanMacro(&tok)
	case isComment(s.char):
		s.scanComment(&tok)
	case isVariable(s.char):
		s.scanVariable(&tok)
	case isDelimiter(s.char):
		s.scanDelim(&tok)
	case s.char == langle && s.peek() == s.char:
		s.scanHeredoc(&tok)
	default:
		s.scanLiteral(&tok)
	}
	if tok.Type == Template {
		s.templateMode = true
	}
	return tok
}

func (s *Scanner) scanTemplate(tok *Token) {
	switch {
	case isVariable(s.char):
		s.scanVariable(tok)
	case s.char == backtick:
		tok.Type = Template
		s.read()
	default:
		s.scanLiteralString(tok)
	}
	if tok.Type == Template {
		s.templateMode = false
	}
}

func (s *Scanner) scanLiteralString(tok *Token) {
	for !s.done() && !isVariable(s.char) && s.char != backtick {
		s.write()
		s.read()
	}
	tok.Type = String
	tok.Literal = s.literal()
}

func (s *Scanner) scanDelim(tok *Token) {
	switch s.char {
	case lcurly:
		tok.Type = BegObj
	case rcurly:
		tok.Type = EndObj
	case lsquare:
		tok.Type = BegArr
	case rsquare:
		tok.Type = EndArr
	case lparen:
		tok.Type = BegGrp
	case rparen:
		tok.Type = EndGrp
	case comma:
		tok.Type = Comma
	case backtick:
		tok.Type = Template
	default:
		tok.Type = Invalid
	}
	s.read()
	switch tok.Type {
	case Template, EndGrp, EndObj, EndArr:
	default:
		s.skipBlank()
	}
}

func (s *Scanner) scanLiteral(tok *Token) {
	for !s.done() && !isBlank(s.char) && !isComment(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.literal()
	switch tok.Literal {
	case "true", "false", "on", "off":
		tok.Type = Boolean
	default:
		tok.Type = Literal
	}
}

func (s *Scanner) scanHex(tok *Token) {
	s.write()
	s.read()
	s.write()
	s.read()
	for !s.done() && isHexa(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Number
	tok.Literal = s.literal()
}

func (s *Scanner) scanOctal(tok *Token) {
	s.write()
	s.read()
	s.write()
	s.read()
	for !s.done() && isOctal(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Number
	tok.Literal = s.literal()
}

func (s *Scanner) scanNumber(tok *Token) {
	if s.char == '0' {
		var next func(*Token)
		if k := s.peek(); k == 'x' {
			next = s.scanHex
		} else if k == 'o' {
			next = s.scanOctal
		} else if isDigit(k) {
			tok.Type = Invalid
			return
		}
		next(tok)
		return
	}
	for !s.done() && isDigit(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Number
	tok.Literal = s.literal()
	if s.char != dot {
		return
	}
	s.write()
	s.read()
	for !s.done() && isDigit(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.literal()
}

func (s *Scanner) scanQuote(tok *Token) {
	quote := s.char
	s.read()
	for !s.done() && s.char != quote {
		s.write()
		s.read()
	}
	tok.Literal = s.literal()
	tok.Type = String
	if s.char != quote {
		tok.Type = Invalid
	} else {
		s.read()
	}
}

func (s *Scanner) scanHeredoc(tok *Token) {
	s.read()
	s.read()

	s.reset()
	for isUpper(s.char) {
		s.write()
		s.read()
	}
	if !isNL(s.char) {
		tok.Type = Invalid
		tok.Literal = s.literal()
		return
	}
	s.read()

	var (
		delim = s.literal()
		tmp   bytes.Buffer
	)
	s.reset()
	for !s.done() {
		for !s.done() && !isNL(s.char) {
			tmp.WriteRune(s.char)
			s.read()
		}
		if tmp.String() == delim {
			break
		}
		for isNL(s.char) {
			tmp.WriteRune(s.char)
			s.read()
		}
		io.Copy(&s.str, &tmp)
		tmp.Reset()
	}
	tok.Type = Heredoc
	tok.Literal = s.literal()
}

func (s *Scanner) scanVariable(tok *Token) {
	env := s.char == arobase
	s.read()
	if !isLetter(s.char) {
		tok.Type = Invalid
		return
	}
	for !s.done() && isAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Type = LocalVar
	if env {
		tok.Type = EnvVar
	}
	tok.Literal = s.literal()
}

func (s *Scanner) scanMacro(tok *Token) {
	s.read()
	for !s.done() && isLetter(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Macro
	tok.Literal = s.literal()
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	s.skip(isSpace)
	for !s.done() && !isNL(s.char) {
		s.write()
		s.read()
	}
	s.skipBlank()
	tok.Type = Comment
	tok.Literal = s.literal()
}

func (s *Scanner) literal() string {
	return s.str.String()
}

func (s *Scanner) reset() {
	s.str.Reset()
}

func (s *Scanner) write() {
	s.str.WriteRune(s.char)
}

func (s *Scanner) read() {
	s.old = s.Position
	if s.char == '\n' {
		s.Column = 0
		s.Line++
	}
	s.Column++
	char, _, err := s.input.ReadRune()
	if errors.Is(err, io.EOF) {
		char = utf8.RuneError
	}
	s.char = char
}

func (s *Scanner) peek() rune {
	defer s.input.UnreadRune()
	r, _, _ := s.input.ReadRune()
	return r
}

func (s *Scanner) done() bool {
	return s.char == utf8.RuneError
}

func (s *Scanner) skip(ok func(rune) bool) {
	for !s.done() && ok(s.char) {
		s.read()
	}
}

func (s *Scanner) skipBlank() {
	for !s.done() && isBlank(s.char) {
		s.read()
	}
}

const (
	space      = ' '
	tab        = '\t'
	cr         = '\r'
	nl         = '\n'
	underscore = '_'
	dash       = '-'
	lcurly     = '{'
	rcurly     = '}'
	lsquare    = '['
	rsquare    = ']'
	lparen     = '('
	rparen     = ')'
	dot        = '.'
	comma      = ','
	squote     = '\''
	dquote     = '"'
	backtick   = '`'
	pound      = '#'
	slash      = '/'
	star       = '*'
	dollar     = '$'
	arobase    = '@'
	langle     = '<'
)

func isDelimiter(r rune) bool {
	return r == rparen || r == lparen || r == lsquare || r == rsquare ||
		r == lcurly || r == rcurly || r == comma || r == backtick
}

func isVariable(r rune) bool {
	return r == dollar || r == arobase
}

func isComment(r rune) bool {
	return r == pound
}

func isMacro(r rune) bool {
	return r == dot
}

func isQuote(r rune) bool {
	return r == squote || r == dquote
}

func isAlpha(r rune) bool {
	return isDigit(r) || isLetter(r)
}

func isLetter(r rune) bool {
	return r == underscore || isUpper(r) || isLower(r)
}

func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isLower(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isOctal(r rune) bool {
	return r >= '0' && r <= '7'
}

func isHexa(r rune) bool {
	return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'Z')
}

func isSpace(r rune) bool {
	return r == space || r == tab
}

func isNL(r rune) bool {
	return r == cr || r == nl
}

func isBlank(r rune) bool {
	return isSpace(r) || isNL(r)
}
