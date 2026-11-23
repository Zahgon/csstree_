package syntaxdef

import "github.com/csstree/csstree/internal/jsutil"

const (
	charTab   = 9
	charN     = 10
	charF     = 12
	charR     = 13
	charSpace = 32
)

// nameChar marks the code points a keyword of a definition may be built from. The original builds
// the same table by testing /[a-zA-Z0-9\-]/ against every code point below 128.
var nameChar = func() [128]bool {
	var table [128]bool
	for code := 0; code < 128; code++ {
		c := byte(code)
		table[code] = (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-'
	}
	return table
}()

// scanner reads a value definition one code unit at a time. Its positions are UTF-16 code unit
// offsets into the definition.
type scanner struct {
	str jsutil.Str
	pos int
}

// syntaxPanic carries a scanner error up to Parse, which is where the original's throw is caught.
type syntaxPanic struct {
	err *jsutil.Error
}

// newSyntaxError builds the error the original's SyntaxError builds: the message on its own, and
// the full message, which repeats the definition and marks the place the scanner stopped at.
func newSyntaxError(message, input jsutil.Str, offset int) *jsutil.Error {
	marked := offset
	if marked == 0 {
		marked = len(input)
	}
	if marked < 0 {
		marked = 0
	}
	line := make(jsutil.Str, 0, marked+3)
	line = append(line, '-', '-')
	for i := 0; i < marked; i++ {
		line = append(line, '-')
	}
	line = append(line, '^')

	full := jsutil.Concat(message, jsutil.FromString("\n  "), input, jsutil.FromString("\n"), line)

	return &jsutil.Error{
		Name:       "SyntaxError",
		RawMessage: message,
		Message:    full,
		Offset:     offset,
	}
}

func newScanner(str jsutil.Str) *scanner {
	return &scanner{str: str}
}

func (s *scanner) charCodeAt(pos int) uint16 {
	if pos < len(s.str) {
		return s.str.CharCodeAt(pos)
	}
	return 0
}

func (s *scanner) charCode() uint16 {
	return s.charCodeAt(s.pos)
}

func (s *scanner) isNameCharCode() bool {
	return s.isNameCharCodeOf(s.charCode())
}

func (s *scanner) isNameCharCodeOf(code uint16) bool {
	return code < 128 && nameChar[code]
}

func (s *scanner) nextCharCode() uint16 {
	return s.charCodeAt(s.pos + 1)
}

func (s *scanner) nextNonWsCode(pos int) uint16 {
	return s.charCodeAt(s.findWsEnd(pos))
}

func (s *scanner) skipWs() {
	s.pos = s.findWsEnd(s.pos)
}

func (s *scanner) findWsEnd(pos int) int {
	for ; pos < len(s.str); pos++ {
		code := s.str.CharCodeAt(pos)
		if code != charR && code != charN && code != charF && code != charSpace && code != charTab {
			break
		}
	}

	return pos
}

func (s *scanner) substringToPos(end int) jsutil.Str {
	result := s.str.Substring(s.pos, end)
	s.pos = end
	return result
}

func (s *scanner) eat(code uint16) {
	if s.charCode() != code {
		s.error(jsutil.Concat(jsutil.FromString("Expect `"), jsutil.Str{code}, jsutil.FromString("`")))
	}

	s.pos++
}

// peek returns the code unit at the current position as a string and steps past it, or an empty
// string at the end of the definition.
func (s *scanner) peek() jsutil.Str {
	if s.pos < len(s.str) {
		result := s.str.Substring(s.pos, s.pos+1)
		s.pos++
		return result
	}
	return jsutil.Str{}
}

func (s *scanner) error(message jsutil.Str) {
	panic(syntaxPanic{err: newSyntaxError(message, s.str, s.pos)})
}

func (s *scanner) errorText(message string) {
	s.error(jsutil.FromString(message))
}

func (s *scanner) scanSpaces() jsutil.Str {
	return s.substringToPos(s.findWsEnd(s.pos))
}

func (s *scanner) scanWord() jsutil.Str {
	end := s.pos

	for ; end < len(s.str); end++ {
		code := s.str.CharCodeAt(end)
		if code >= 128 || !nameChar[code] {
			break
		}
	}

	if s.pos == end {
		s.errorText("Expect a keyword")
	}

	return s.substringToPos(end)
}

func (s *scanner) scanNumber() jsutil.Str {
	end := s.pos

	for ; end < len(s.str); end++ {
		code := s.str.CharCodeAt(end)

		if code < 48 || code > 57 {
			break
		}
	}

	if s.pos == end {
		s.errorText("Expect a number")
	}

	return s.substringToPos(end)
}

func (s *scanner) scanString() jsutil.Str {
	end := s.str.IndexOfFrom(jsutil.FromString("'"), s.pos+1)

	if end == -1 {
		s.pos = len(s.str)
		s.errorText("Expect an apostrophe")
	}

	return s.substringToPos(end + 1)
}
