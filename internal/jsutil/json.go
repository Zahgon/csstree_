package jsutil

import "strconv"

// Value is a JSON value as JavaScript holds it before JSON.stringify runs: an object keeps the
// order its properties were added in, which is the order the original writes them in, so the JSON
// text is the same text.
type Value interface {
	writeJSON(buf *[]uint16, indent, depth int)
}

// Null is the JSON null.
type Null struct{}

// Bool is a JSON true or false.
type Bool bool

// Num is a JSON number held as a double, printed the way JavaScript prints a number.
type Num float64

// Int is a JSON number that is an integer.
type Int int

// JSONString is a JSON string.
type JSONString Str

// Array is a JSON array.
type Array []Value

// Object is a JSON object that remembers the order of its keys.
type Object struct {
	keys   []Str
	values []Value
}

// NewObject returns an empty object.
func NewObject() *Object {
	return &Object{}
}

// Set adds a key, or replaces the value of a key that is already there without moving it, which is
// how assigning to a property of a JavaScript object behaves.
func (o *Object) Set(key Str, value Value) *Object {
	for i, k := range o.keys {
		if k.Equal(key) {
			o.values[i] = value
			return o
		}
	}
	o.keys = append(o.keys, key)
	o.values = append(o.values, value)
	return o
}

// SetString adds a key given as an ASCII literal.
func (o *Object) SetString(key string, value Value) *Object {
	return o.Set(FromString(key), value)
}

// Len is the number of keys.
func (o *Object) Len() int {
	return len(o.keys)
}

// Keys returns the keys in their order.
func (o *Object) Keys() []Str {
	return o.keys
}

// Get returns the value of a key and whether it is there.
func (o *Object) Get(key Str) (Value, bool) {
	for i, k := range o.keys {
		if k.Equal(key) {
			return o.values[i], true
		}
	}
	return nil, false
}

// Stringify is JSON.stringify(value, null, indent).
func Stringify(value Value, indent int) Str {
	buf := make([]uint16, 0, 256)
	value.writeJSON(&buf, indent, 0)
	return Str(buf)
}

func appendASCII(buf *[]uint16, s string) {
	for i := 0; i < len(s); i++ {
		*buf = append(*buf, uint16(s[i]))
	}
}

func appendNewlineIndent(buf *[]uint16, indent, depth int) {
	if indent <= 0 {
		return
	}
	*buf = append(*buf, '\n')
	for i := 0; i < indent*depth; i++ {
		*buf = append(*buf, ' ')
	}
}

func (Null) writeJSON(buf *[]uint16, indent, depth int) { appendASCII(buf, "null") }

func (b Bool) writeJSON(buf *[]uint16, indent, depth int) {
	if b {
		appendASCII(buf, "true")
	} else {
		appendASCII(buf, "false")
	}
}

func (n Num) writeJSON(buf *[]uint16, indent, depth int) {
	appendASCII(buf, NumberToString(float64(n)))
}

func (n Int) writeJSON(buf *[]uint16, indent, depth int) {
	appendASCII(buf, strconv.Itoa(int(n)))
}

const hexDigits = "0123456789abcdef"

func (s JSONString) writeJSON(buf *[]uint16, indent, depth int) {
	*buf = append(*buf, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			appendASCII(buf, "\\\"")
		case c == '\\':
			appendASCII(buf, "\\\\")
		case c == 0x08:
			appendASCII(buf, "\\b")
		case c == 0x09:
			appendASCII(buf, "\\t")
		case c == 0x0A:
			appendASCII(buf, "\\n")
		case c == 0x0C:
			appendASCII(buf, "\\f")
		case c == 0x0D:
			appendASCII(buf, "\\r")
		case c < 0x20:
			appendASCII(buf, "\\u00")
			*buf = append(*buf, uint16(hexDigits[c>>4]), uint16(hexDigits[c&0xF]))
		case c >= 0xD800 && c <= 0xDBFF && i+1 < len(s) && s[i+1] >= 0xDC00 && s[i+1] <= 0xDFFF:
			// A surrogate pair stays as it is.
			*buf = append(*buf, c, s[i+1])
			i++
		case c >= 0xD800 && c <= 0xDFFF:
			// A code unit that is not part of a pair is escaped, as well formed JSON requires.
			appendASCII(buf, "\\u")
			*buf = append(*buf, uint16(hexDigits[(c>>12)&0xF]), uint16(hexDigits[(c>>8)&0xF]),
				uint16(hexDigits[(c>>4)&0xF]), uint16(hexDigits[c&0xF]))
		default:
			*buf = append(*buf, c)
		}
	}
	*buf = append(*buf, '"')
}

func (a Array) writeJSON(buf *[]uint16, indent, depth int) {
	if len(a) == 0 {
		appendASCII(buf, "[]")
		return
	}
	*buf = append(*buf, '[')
	for i, item := range a {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		appendNewlineIndent(buf, indent, depth+1)
		if item == nil {
			appendASCII(buf, "null")
			continue
		}
		item.writeJSON(buf, indent, depth+1)
	}
	appendNewlineIndent(buf, indent, depth)
	*buf = append(*buf, ']')
}

func (o *Object) writeJSON(buf *[]uint16, indent, depth int) {
	if o == nil || len(o.keys) == 0 {
		appendASCII(buf, "{}")
		return
	}
	*buf = append(*buf, '{')
	for i, key := range o.keys {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		appendNewlineIndent(buf, indent, depth+1)
		JSONString(key).writeJSON(buf, indent, depth+1)
		*buf = append(*buf, ':')
		if indent > 0 {
			*buf = append(*buf, ' ')
		}
		if o.values[i] == nil {
			appendASCII(buf, "null")
			continue
		}
		o.values[i].writeJSON(buf, indent, depth+1)
	}
	appendNewlineIndent(buf, indent, depth)
	*buf = append(*buf, '}')
}

// Delete removes a key and its value, leaving the order of the rest alone.
func (o *Object) Delete(key Str) {
	for i, k := range o.keys {
		if k.Equal(key) {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			o.values = append(o.values[:i], o.values[i+1:]...)
			return
		}
	}
}

// Has reports whether the key is there.
func (o *Object) Has(key Str) bool {
	for _, k := range o.keys {
		if k.Equal(key) {
			return true
		}
	}
	return false
}
