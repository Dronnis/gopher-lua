package parse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/yuin/gopher-lua/ast"
)

const EOF = -1
const whitespace1 = 1<<'\t' | 1<<'\n' | 1<<'\f' | 1<<'\r' | 1<<'\v' | 1<<' '
const whitespace2 = 1<<'\t' | 1<<'\n' | 1<<'\f' | 1<<'\r' | 1<<'\v' | 1<<' '

type Error struct {
	Pos     ast.Position
	Message string
	Token   string
}

func (e *Error) Error() string {
	pos := e.Pos
	// Lua 5.3 compatible error format: [source]:line: message near token
	source := pos.Source
	// Wrap non-file sources in [string "..."] format
	if len(source) > 0 && source[0] != '/' && source[0] != '[' {
		source = fmt.Sprintf("[string \"%s\"]", source)
	}
	// For EOF errors, use the last line number seen
	line := pos.Line
	if line <= 0 || line == EOF {
		if pos.LastLine > 0 {
			line = pos.LastLine
		} else {
			line = 1
		}
	}
	// If message already contains "near", don't add token
	if strings.Contains(e.Message, " near ") {
		return fmt.Sprintf("%s:%d: %s", source, line, e.Message)
	}
	return fmt.Sprintf("%s:%d: %s near '%s'", source, line, e.Message, e.Token)
}

func writeChar(buf *bytes.Buffer, c int) { buf.WriteByte(byte(c)) }

func isDecimal(ch int) bool { return '0' <= ch && ch <= '9' }

func isOctal(ch int) bool { return '0' <= ch && ch <= '7' }

func isHex(ch int) bool {
	return '0' <= ch && ch <= '9' || 'a' <= ch && ch <= 'f' || 'A' <= ch && ch <= 'F'
}

func isIdent(ch int, pos int) bool {
	return ch == '_' || 'A' <= ch && ch <= 'Z' || 'a' <= ch && ch <= 'z' || isDecimal(ch) && pos > 0
}

func isDigit(ch int) bool {
	return '0' <= ch && ch <= '9' || 'a' <= ch && ch <= 'f' || 'A' <= ch && ch <= 'F'
}

type Scanner struct {
	Pos      ast.Position
	LastLine int
	reader   *bufio.Reader
}

func NewScanner(reader io.Reader, source string) *Scanner {
	return &Scanner{
		Pos: ast.Position{
			Source: source,
			Line:   1,
			Column: 0,
		},
		LastLine: 1,
		reader:   bufio.NewReaderSize(reader, 4096),
	}
}

func (sc *Scanner) Error(tok string, msg string) *Error {
	return &Error{ast.Position{Source: sc.Pos.Source, Line: sc.Pos.Line, Column: sc.Pos.Column, LastLine: sc.LastLine}, msg, tok}
}

func (sc *Scanner) TokenError(tok ast.Token, msg string) *Error { return &Error{tok.Pos, msg, tok.Str} }

func (sc *Scanner) readNext() int {
	ch, err := sc.reader.ReadByte()
	if err == io.EOF {
		return EOF
	}
	return int(ch)
}

func (sc *Scanner) Newline(ch int) {
	if ch < 0 {
		return
	}
	sc.Pos.Line += 1
	sc.Pos.Column = 0
	sc.LastLine = sc.Pos.Line
	next := sc.Peek()
	if ch == '\n' && next == '\r' || ch == '\r' && next == '\n' {
		sc.reader.ReadByte()
	}
}

func (sc *Scanner) Next() int {
	ch := sc.readNext()
	switch ch {
	case '\n', '\r':
		sc.Newline(ch)
		ch = int('\n')
	case EOF:
		sc.Pos.Line = EOF
		sc.Pos.Column = 0
	default:
		sc.Pos.Column++
	}
	return ch
}

func (sc *Scanner) Peek() int {
	ch := sc.readNext()
	if ch != EOF {
		sc.reader.UnreadByte()
	}
	return ch
}

func (sc *Scanner) skipWhiteSpace(whitespace int64) int {
	ch := sc.Next()
	for ; whitespace&(1<<uint(ch)) != 0; ch = sc.Next() {
	}
	return ch
}

func (sc *Scanner) skipComments(ch int) error {
	// multiline comment
	if sc.Peek() == '[' {
		ch = sc.Next()
		if sc.Peek() == '[' || sc.Peek() == '=' {
			var buf bytes.Buffer
			if err := sc.scanMultilineString(sc.Next(), &buf); err != nil {
				return sc.Error(buf.String(), "invalid multiline comment")
			}
			return nil
		}
	}
	for {
		if ch == '\n' || ch == '\r' || ch < 0 {
			break
		}
		ch = sc.Next()
	}
	return nil
}

func (sc *Scanner) scanIdent(ch int, buf *bytes.Buffer) error {
	writeChar(buf, ch)
	for isIdent(sc.Peek(), 1) {
		writeChar(buf, sc.Next())
	}
	return nil
}

func (sc *Scanner) scanDecimal(ch int, buf *bytes.Buffer) error {
	writeChar(buf, ch)
	for isDecimal(sc.Peek()) {
		writeChar(buf, sc.Next())
	}
	return nil
}

func (sc *Scanner) scanNumber(ch int, buf *bytes.Buffer) error {
	if ch == '0' { // octal
		if sc.Peek() == 'x' || sc.Peek() == 'X' {
			writeChar(buf, ch)
			writeChar(buf, sc.Next())
			hasvalue := false
			// Lua 5.3: hex numbers can start with 0x.digit (e.g., 0x.41)
			if sc.Peek() == '.' {
				writeChar(buf, sc.Next())
				for isHex(sc.Peek()) {
					writeChar(buf, sc.Next())
					hasvalue = true
				}
				// Lua 5.3: hex numbers can have binary exponent p/P
				if sc.Peek() == 'p' || sc.Peek() == 'P' {
					writeChar(buf, sc.Next())
					if sc.Peek() == '-' || sc.Peek() == '+' {
						writeChar(buf, sc.Next())
					}
					sc.scanDecimal(sc.Next(), buf)
				}
				if !hasvalue {
					return sc.Error(buf.String(), "illegal hexadecimal number")
				}
				return nil
			}
			// Lua 5.3: hex numbers can have fractional part (e.g., 0x10.41p2)
			for isDigit(sc.Peek()) {
				writeChar(buf, sc.Next())
				hasvalue = true
			}
			if sc.Peek() == '.' {
				writeChar(buf, sc.Next())
				for isHex(sc.Peek()) {
					writeChar(buf, sc.Next())
				}
			}
			// Lua 5.3: hex numbers can have binary exponent p/P
			if sc.Peek() == 'p' || sc.Peek() == 'P' {
				writeChar(buf, sc.Next())
				if sc.Peek() == '-' || sc.Peek() == '+' {
					writeChar(buf, sc.Next())
				}
				sc.scanDecimal(sc.Next(), buf)
			}
			if !hasvalue {
				return sc.Error(buf.String(), "illegal hexadecimal number")
			}
			return nil
		} else if sc.Peek() != '.' && isDecimal(sc.Peek()) {
			ch = sc.Next()
		}
	}
	sc.scanDecimal(ch, buf)
	if sc.Peek() == '.' {
		sc.scanDecimal(sc.Next(), buf)
	}
	if ch = sc.Peek(); ch == 'e' || ch == 'E' {
		writeChar(buf, sc.Next())
		if ch = sc.Peek(); ch == '-' || ch == '+' {
			writeChar(buf, sc.Next())
		}
		sc.scanDecimal(sc.Next(), buf)
	}

	return nil
}

func (sc *Scanner) scanString(quote int, buf *bytes.Buffer) error {
	ch := sc.Next()
	for ch != quote {
		if ch == '\n' || ch == '\r' || ch < 0 {
			return sc.Error(buf.String(), "unterminated string")
		}
		if ch == '\\' {
			if err := sc.scanEscape(ch, buf); err != nil {
				return err
			}
		} else {
			writeChar(buf, ch)
		}
		ch = sc.Next()
	}
	return nil
}

func (sc *Scanner) scanEscape(ch int, buf *bytes.Buffer) error {
	ch = sc.Next()
	switch ch {
	case 'a':
		buf.WriteByte('\a')
	case 'b':
		buf.WriteByte('\b')
	case 'f':
		buf.WriteByte('\f')
	case 'n':
		buf.WriteByte('\n')
	case 'r':
		buf.WriteByte('\r')
	case 't':
		buf.WriteByte('\t')
	case 'v':
		buf.WriteByte('\v')
	case '\\':
		buf.WriteByte('\\')
	case '"':
		buf.WriteByte('"')
	case '\'':
		buf.WriteByte('\'')
	case 'z':
		// \z skips all following whitespace characters (Lua 5.3 feature)
		for {
			ch = sc.Peek()
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\f' || ch == '\v' {
				sc.Next()
			} else {
				break
			}
		}
	case '\n':
		buf.WriteByte('\n')
	case '\r':
		buf.WriteByte('\n')
		sc.Newline('\r')
	case 'x':
		// \xXX hex escape (Lua 5.3 feature)
		// Save position before reading hex digits for error reporting
		savedPos := sc.Pos
		h1 := sc.Next()
		h2 := sc.Next()
		if isHex(h1) && isHex(h2) {
			hex := string([]byte{byte(h1), byte(h2)})
			val, _ := strconv.ParseInt(hex, 16, 32)
			buf.WriteByte(byte(val))
		} else {
			// Invalid hex escape - build token based on what we read
			// Lua 5.3 behavior:
			// - If h1 is not hex: token is \xh1 (e.g., \xr, \x")
			// - If h1 is hex but h2 is not: token is \xh1h2 (e.g., \x5", \x8%)
			token := "\\x"
			if h1 != EOF {
				token += string(rune(h1))
				// Always include h2 if it exists (Lua 5.3 behavior)
				// This provides better context for error messages
				if h2 != EOF {
					token += string(rune(h2))
				}
			}
			// For EOF cases, use the saved position for proper error format
			if sc.Pos.Line == EOF {
				sc.Pos = savedPos
			}
			return sc.Error(token, "invalid hex escape sequence")
		}
	case 'u':
		// \u{XXX} Unicode escape (Lua 5.3 feature)
		if sc.Peek() == '{' {
			sc.Next() // skip '{'
			hex := ""
			for {
				ch = sc.Peek()
				if isHex(ch) {
					hex += string([]byte{byte(sc.Next())})
				} else if ch == '}' {
					sc.Next() // skip '}'
					break
				} else if ch == EOF {
					// Unexpected EOF inside \u{} - include prefix and partial hex
					token := buf.String() + "\\u{" + hex
					return sc.Error(token, "unfinished \\u{} escape sequence")
				} else {
					// Invalid character in \u{} - read it and include in token
					invalidCh := sc.Next()
					token := buf.String() + "\\u{" + hex + string(rune(invalidCh))
					// Check if next char is '}' and include it
					if sc.Peek() == '}' {
						sc.Next() // skip '}'
						token += "}"
					}
					return sc.Error(token, "invalid unicode escape sequence")
				}
			}
			if len(hex) > 0 {
				val, _ := strconv.ParseInt(hex, 16, 32)
				// Check for valid Unicode code point range
				if val > 0x10FFFF {
					// For invalid code point, include the string prefix and full escape sequence
					token := buf.String() + "\\u{" + hex + "}"
					return sc.Error(token, "invalid unicode code point")
				}
				// Encode as UTF-8
				if val <= 0x7F {
					buf.WriteByte(byte(val))
				} else if val <= 0x7FF {
					buf.WriteByte(0xC0 | byte(val>>6))
					buf.WriteByte(0x80 | byte(val&0x3F))
				} else if val <= 0xFFFF {
					buf.WriteByte(0xE0 | byte(val>>12))
					buf.WriteByte(0x80 | byte((val>>6)&0x3F))
					buf.WriteByte(0x80 | byte(val&0x3F))
				} else if val <= 0x10FFFF {
					buf.WriteByte(0xF0 | byte(val>>18))
					buf.WriteByte(0x80 | byte((val>>12)&0x3F))
					buf.WriteByte(0x80 | byte((val>>6)&0x3F))
					buf.WriteByte(0x80 | byte(val&0x3F))
				}
			} else {
				// No hex digits - include prefix and opening brace
				token := buf.String() + "\\u{"
				return sc.Error(token, "invalid unicode escape sequence")
			}
		} else {
			// Not a Unicode escape (no '{' after \u) - return error
			// Include following characters to match Lua 5.3 error format
			token := buf.String() + "\\u"
			nextCh := sc.Peek()
			if nextCh != EOF {
				token += string(rune(nextCh))
			}
			return sc.Error(token, "invalid unicode escape sequence")
		}
	default:
		if '0' <= ch && ch <= '9' {
			// Decimal escape sequence (\0 to \255)
			// Lua 5.3 uses decimal, not octal, for \ddd escapes
			bytes := []byte{byte(ch)}
			for i := 0; i < 2 && isDecimal(sc.Peek()); i++ {
				bytes = append(bytes, byte(sc.Next()))
			}
			val, _ := strconv.ParseInt(string(bytes), 10, 32)
			if val > 255 {
				// Value > 255 is an error in Lua 5.3
				token := "\\" + string(bytes)
				nextCh := sc.Peek()
				if nextCh != EOF {
					token += string(rune(nextCh))
				}
				return sc.Error(token, "invalid escape sequence")
			}
			writeChar(buf, int(val))
		} else {
			// Invalid escape sequence - Lua 5.3 raises an error for unknown escapes
			return sc.Error("\\"+string(rune(ch)), "invalid escape sequence")
		}
	}
	return nil
}

func (sc *Scanner) countSep(ch int) (int, int) {
	count := 0
	for ; ch == '='; count = count + 1 {
		ch = sc.Next()
	}
	return count, ch
}

func (sc *Scanner) scanMultilineString(ch int, buf *bytes.Buffer) error {
	var count1, count2 int
	count1, ch = sc.countSep(ch)
	if ch != '[' {
		return sc.Error(string(rune(ch)), "invalid multiline string")
	}
	ch = sc.Next()
	if ch == '\n' || ch == '\r' {
		ch = sc.Next()
	}
	for {
		if ch < 0 {
			return sc.Error(buf.String(), "unterminated multiline string")
		} else if ch == ']' {
			count2, ch = sc.countSep(sc.Next())
			if count1 == count2 && ch == ']' {
				goto finally
			}
			buf.WriteByte(']')
			buf.WriteString(strings.Repeat("=", count2))
			continue
		}
		writeChar(buf, ch)
		ch = sc.Next()
	}

finally:
	return nil
}

var reservedWords = map[string]int{
	"and": TAnd, "break": TBreak, "do": TDo, "else": TElse, "elseif": TElseIf,
	"end": TEnd, "false": TFalse, "for": TFor, "function": TFunction,
	"if": TIf, "in": TIn, "local": TLocal, "nil": TNil, "not": TNot, "or": TOr,
	"return": TReturn, "repeat": TRepeat, "then": TThen, "true": TTrue,
	"until": TUntil, "while": TWhile, "goto": TGoto}

func (sc *Scanner) Scan(lexer *Lexer) (ast.Token, error) {
redo:
	var err error
	tok := ast.Token{}
	newline := false

	ch := sc.skipWhiteSpace(whitespace1)
	if ch == '\n' || ch == '\r' {
		newline = true
		ch = sc.skipWhiteSpace(whitespace2)
	}

	if ch == '(' && lexer.PrevTokenType == ')' {
		lexer.PNewLine = newline
	} else {
		lexer.PNewLine = false
	}

	var _buf bytes.Buffer
	buf := &_buf
	tok.Pos = sc.Pos

	switch {
	case isIdent(ch, 0):
		tok.Type = TIdent
		err = sc.scanIdent(ch, buf)
		tok.Str = buf.String()
		if err != nil {
			goto finally
		}
		if typ, ok := reservedWords[tok.Str]; ok {
			tok.Type = typ
		}
	case isDecimal(ch):
		tok.Type = TNumber
		err = sc.scanNumber(ch, buf)
		tok.Str = buf.String()
	default:
		switch ch {
		case EOF:
			tok.Type = EOF
		case '-':
			if sc.Peek() == '-' {
				err = sc.skipComments(sc.Next())
				if err != nil {
					goto finally
				}
				goto redo
			} else {
				tok.Type = ch
				tok.Str = string(rune(ch))
			}
		case '"', '\'':
			tok.Type = TString
			err = sc.scanString(ch, buf)
			tok.Str = buf.String()
		case '[':
			if c := sc.Peek(); c == '[' || c == '=' {
				tok.Type = TString
				err = sc.scanMultilineString(sc.Next(), buf)
				tok.Str = buf.String()
			} else {
				tok.Type = ch
				tok.Str = string(rune(ch))
			}
		case '=':
			if sc.Peek() == '=' {
				tok.Type = TEqeq
				tok.Str = "=="
				sc.Next()
			} else {
				tok.Type = ch
				tok.Str = string(rune(ch))
			}
		case '~':
			if sc.Peek() == '=' {
				tok.Type = TNeq
				tok.Str = "~="
				sc.Next()
			} else {
				tok.Type = Ttilde
				tok.Str = "~"
			}
		case '<':
			if sc.Peek() == '=' {
				tok.Type = TLte
				tok.Str = "<="
				sc.Next()
			} else if sc.Peek() == '<' {
				tok.Type = T2LessThan
				tok.Str = "<<"
				sc.Next()
			} else {
				tok.Type = ch
				tok.Str = string(rune(ch))
			}
		case '>':
			if sc.Peek() == '=' {
				tok.Type = TGte
				tok.Str = ">="
				sc.Next()
			} else if sc.Peek() == '>' {
				tok.Type = T2GreaterThan
				tok.Str = ">>"
				sc.Next()
			} else {
				tok.Type = ch
				tok.Str = string(rune(ch))
			}
		case '.':
			ch2 := sc.Peek()
			switch {
			case isDecimal(ch2):
				tok.Type = TNumber
				err = sc.scanNumber(ch, buf)
				tok.Str = buf.String()
			case ch2 == '.':
				writeChar(buf, ch)
				writeChar(buf, sc.Next())
				if sc.Peek() == '.' {
					writeChar(buf, sc.Next())
					tok.Type = T3Comma
				} else {
					tok.Type = T2Comma
				}
			default:
				tok.Type = '.'
			}
			tok.Str = buf.String()
		case ':':
			if sc.Peek() == ':' {
				tok.Type = T2Colon
				tok.Str = "::"
				sc.Next()
			} else {
				tok.Type = ch
				tok.Str = string(rune(ch))
			}
		case '/':
			if sc.Peek() == '/' {
				tok.Type = T2Slash
				tok.Str = "//"
				sc.Next()
			} else {
				tok.Type = ch
				tok.Str = string(rune(ch))
			}
		case '|':
			tok.Type = Tpipe
			tok.Str = "|"
		case '&':
			if sc.Peek() == '&' {
				tok.Type = TAnd
				tok.Str = "&&"
				sc.Next()
			} else {
				tok.Type = Tampersand
				tok.Str = "&"
			}
		case '+', '*', '%', '^', '#', '(', ')', '{', '}', ']', ';', ',':
			tok.Type = ch
			tok.Str = string(rune(ch))
		default:
			writeChar(buf, ch)
			err = sc.Error(buf.String(), "unexpected symbol near '"+buf.String()+"'")
			goto finally
		}
	}

finally:
	tok.Name = TokenName(int(tok.Type))
	return tok, err
}

// yacc interface {{{

type Lexer struct {
	scanner       *Scanner
	Stmts         []ast.Stmt
	PNewLine      bool
	Token         ast.Token
	PrevTokenType int
}

func (lx *Lexer) Lex(lval *yySymType) int {
	lx.PrevTokenType = lx.Token.Type
	tok, err := lx.scanner.Scan(lx)
	if err != nil {
		panic(err)
	}
	if tok.Type < 0 {
		return 0
	}
	lval.token = tok
	lx.Token = tok
	return int(tok.Type)
}

func (lx *Lexer) Error(message string) {
	panic(lx.scanner.Error(lx.Token.Str, message))
}

func (lx *Lexer) TokenError(tok ast.Token, message string) {
	panic(lx.scanner.TokenError(tok, message))
}

// skipShebang пропускает shebang строку (#!...) или комментарий (#...)
// если они находятся в начале файла. Это необходимо для совместимости с Lua 5.3
// и Unix-скриптами.
func (lx *Lexer) skipShebang() {
	// Проверяем первый символ
	ch1 := lx.scanner.Next()
	if ch1 != '#' {
		// Это не комментарий с #, возвращаем символ обратно
		if ch1 != EOF {
			lx.scanner.reader.UnreadByte()
		}
		return
	}

	// Если это shebang (#!) или просто комментарий (#...) в начале файла,
	// пропускаем до конца строки
	// В Lua 5.3+ первая строка начинающаяся с # игнорируется (для совместимости с shell)
	for {
		ch := lx.scanner.Next()
		if ch == '\n' || ch == EOF {
			break
		}
	}
}

func Parse(reader io.Reader, name string) (chunk []ast.Stmt, err error) {
	lexer := &Lexer{NewScanner(reader, name), nil, false, ast.Token{Str: ""}, TNil}
	chunk = nil
	defer func() {
		if e := recover(); e != nil {
			err, _ = e.(error)
		}
	}()
	// Lua 5.3: skip shebang line on first line (#!...)
	lexer.skipShebang()
	yyParse(lexer)
	chunk = lexer.Stmts
	return
}

// }}}

// Dump {{{

func isInlineDumpNode(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Struct, reflect.Slice, reflect.Interface, reflect.Ptr:
		return false
	default:
		return true
	}
}

func dump(node interface{}, level int, s string) string {
	rt := reflect.TypeOf(node)
	if fmt.Sprint(rt) == "<nil>" {
		return strings.Repeat(s, level) + "<nil>"
	}

	rv := reflect.ValueOf(node)
	buf := []string{}
	switch rt.Kind() {
	case reflect.Slice:
		if rv.Len() == 0 {
			return strings.Repeat(s, level) + "<empty>"
		}
		for i := 0; i < rv.Len(); i++ {
			buf = append(buf, dump(rv.Index(i).Interface(), level, s))
		}
	case reflect.Ptr:
		vt := rv.Elem()
		tt := rt.Elem()
		indicies := []int{}
		for i := 0; i < tt.NumField(); i++ {
			if strings.Index(tt.Field(i).Name, "Base") > -1 {
				continue
			}
			indicies = append(indicies, i)
		}
		switch {
		case len(indicies) == 0:
			return strings.Repeat(s, level) + "<empty>"
		case len(indicies) == 1 && isInlineDumpNode(vt.Field(indicies[0])):
			for _, i := range indicies {
				buf = append(buf, strings.Repeat(s, level)+"- Node$"+tt.Name()+": "+dump(vt.Field(i).Interface(), 0, s))
			}
		default:
			buf = append(buf, strings.Repeat(s, level)+"- Node$"+tt.Name())
			for _, i := range indicies {
				if isInlineDumpNode(vt.Field(i)) {
					inf := dump(vt.Field(i).Interface(), 0, s)
					buf = append(buf, strings.Repeat(s, level+1)+tt.Field(i).Name+": "+inf)
				} else {
					buf = append(buf, strings.Repeat(s, level+1)+tt.Field(i).Name+": ")
					buf = append(buf, dump(vt.Field(i).Interface(), level+2, s))
				}
			}
		}
	default:
		buf = append(buf, strings.Repeat(s, level)+fmt.Sprint(node))
	}
	return strings.Join(buf, "\n")
}

func Dump(chunk []ast.Stmt) string {
	return dump(chunk, 0, "   ")
}

// }}
