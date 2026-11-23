package jsutil

import (
	"fmt"
	"strconv"
	"unicode/utf16"
)

// ParseJSON reads JSON text the way JSON.parse reads it: an object keeps the order its keys first
// appeared in, a repeated key keeps its first place and its last value, and every number is a
// double. The text is read as UTF-16 code units, so an escaped surrogate stays as it was written.
func ParseJSON(text Str) (Value, error) {
	p := &jsonParser{src: text}
	p.skipSpace()
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos != len(p.src) {
		return nil, fmt.Errorf("unexpected token at position %d", p.pos)
	}
	return value, nil
}

// ParseJSONBytes reads UTF-8 JSON text.
func ParseJSONBytes(b []byte) (Value, error) {
	return ParseJSON(FromUTF8(b))
}

type jsonParser struct {
	src Str
	pos int
}

func (p *jsonParser) skipSpace() {
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *jsonParser) parseValue() (Value, error) {
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("unexpected end of JSON input")
	}
	switch c := p.src[p.pos]; {
	case c == '{':
		return p.parseObject()
	case c == '[':
		return p.parseArray()
	case c == '"':
		s, err := p.parseString()
		if err != nil {
			return nil, err
		}
		return JSONString(s), nil
	case p.literal("true"):
		return Bool(true), nil
	case p.literal("false"):
		return Bool(false), nil
	case p.literal("null"):
		return Null{}, nil
	case c == '-' || (c >= '0' && c <= '9'):
		return p.parseNumber()
	default:
		return nil, fmt.Errorf("unexpected token at position %d", p.pos)
	}
}

func (p *jsonParser) literal(word string) bool {
	if p.pos+len(word) > len(p.src) {
		return false
	}
	for i := 0; i < len(word); i++ {
		if p.src[p.pos+i] != uint16(word[i]) {
			return false
		}
	}
	p.pos += len(word)
	return true
}

func (p *jsonParser) parseObject() (Value, error) {
	p.pos++ // {
	object := NewObject()
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == '}' {
		p.pos++
		return object, nil
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != '"' {
			return nil, fmt.Errorf("expected a key at position %d", p.pos)
		}
		key, err := p.parseString()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != ':' {
			return nil, fmt.Errorf("expected \":\" at position %d", p.pos)
		}
		p.pos++
		p.skipSpace()
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		object.Set(key, value)
		p.skipSpace()
		if p.pos < len(p.src) && p.src[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.pos < len(p.src) && p.src[p.pos] == '}' {
			p.pos++
			return object, nil
		}
		return nil, fmt.Errorf("expected \",\" or \"}\" at position %d", p.pos)
	}
}

func (p *jsonParser) parseArray() (Value, error) {
	p.pos++ // [
	items := Array{}
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == ']' {
		p.pos++
		return items, nil
	}
	for {
		p.skipSpace()
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		items = append(items, value)
		p.skipSpace()
		if p.pos < len(p.src) && p.src[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.pos < len(p.src) && p.src[p.pos] == ']' {
			p.pos++
			return items, nil
		}
		return nil, fmt.Errorf("expected \",\" or \"]\" at position %d", p.pos)
	}
}

func (p *jsonParser) parseString() (Str, error) {
	p.pos++ // "
	out := make(Str, 0, 16)
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == '"':
			p.pos++
			return out, nil
		case c == '\\':
			p.pos++
			if p.pos >= len(p.src) {
				return nil, fmt.Errorf("unexpected end of JSON input")
			}
			switch p.src[p.pos] {
			case '"', '\\', '/':
				out = append(out, p.src[p.pos])
			case 'b':
				out = append(out, 0x08)
			case 'f':
				out = append(out, 0x0C)
			case 'n':
				out = append(out, 0x0A)
			case 'r':
				out = append(out, 0x0D)
			case 't':
				out = append(out, 0x09)
			case 'u':
				if p.pos+4 >= len(p.src) {
					return nil, fmt.Errorf("bad escape at position %d", p.pos)
				}
				value := 0
				for i := 1; i <= 4; i++ {
					d := hexValue(p.src[p.pos+i])
					if d < 0 {
						return nil, fmt.Errorf("bad escape at position %d", p.pos)
					}
					value = value<<4 | d
				}
				out = append(out, uint16(value))
				p.pos += 4
			default:
				return nil, fmt.Errorf("bad escape at position %d", p.pos)
			}
			p.pos++
		case c < 0x20:
			return nil, fmt.Errorf("a control character is not allowed in a string at position %d", p.pos)
		default:
			out = append(out, c)
			p.pos++
		}
	}
	return nil, fmt.Errorf("unexpected end of JSON input")
}

func hexValue(c uint16) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func (p *jsonParser) parseNumber() (Value, error) {
	start := p.pos
	if p.pos < len(p.src) && p.src[p.pos] == '-' {
		p.pos++
	}
	for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
		p.pos++
	}
	if p.pos < len(p.src) && p.src[p.pos] == '.' {
		p.pos++
		for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
			p.pos++
		}
	}
	if p.pos < len(p.src) && (p.src[p.pos] == 'e' || p.src[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.src) && (p.src[p.pos] == '+' || p.src[p.pos] == '-') {
			p.pos++
		}
		for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
			p.pos++
		}
	}
	text := p.src.Substring(start, p.pos).String()
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, fmt.Errorf("bad number at position %d", start)
	}
	if value == float64(int(value)) && text == strconv.Itoa(int(value)) {
		return Int(int(value)), nil
	}
	return Num(value), nil
}

// AsString returns the text of a JSON string value.
func AsString(v Value) (Str, bool) {
	s, ok := v.(JSONString)
	return Str(s), ok
}

// RuneCount is the number of code points a string holds, for the places that count characters
// rather than code units.
func (s Str) RuneCount() int {
	return len(utf16.Decode(s))
}
