// Package jsutil carries the JavaScript semantics the original css-tree implementation shows in
// its results: strings as sequences of UTF-16 code units, the lower casing JavaScript applies to
// a keyword, the JSON text JSON.stringify produces, and the number parsing and printing of the
// places that read a numeric value out of CSS text.
package jsutil

import (
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Str is a JavaScript string: a sequence of UTF-16 code units. CSS source offsets are code unit
// offsets into it, so every position the parser records and every length the utilities measure is
// an index into a Str rather than a byte index.
type Str []uint16

// FromUTF8 decodes UTF-8 bytes the way a JavaScript runtime decodes a file or a stream read as
// text: every byte sequence that is not well formed UTF-8 becomes one U+FFFD per invalid byte.
func FromUTF8(b []byte) Str {
	s := make(Str, 0, len(b))
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size <= 1 {
			s = append(s, 0xFFFD)
			i++
			continue
		}
		if r > 0xFFFF {
			r1, r2 := utf16.EncodeRune(r)
			s = append(s, uint16(r1), uint16(r2))
		} else {
			s = append(s, uint16(r))
		}
		i += size
	}
	return s
}

// FromString decodes a Go string, which is already well formed UTF-8 in practice, the same way.
func FromString(s string) Str {
	return FromUTF8([]byte(s))
}

// UTF8 encodes the string the way a JavaScript runtime writes a string to a stream: a code unit
// that is not part of a surrogate pair becomes U+FFFD.
func (s Str) UTF8() []byte {
	out := make([]byte, 0, len(s)+len(s)/2)
	var buf [4]byte
	for i := 0; i < len(s); i++ {
		c := rune(s[i])
		switch {
		case c >= 0xD800 && c <= 0xDBFF && i+1 < len(s) && s[i+1] >= 0xDC00 && s[i+1] <= 0xDFFF:
			n := utf8.EncodeRune(buf[:], utf16.DecodeRune(c, rune(s[i+1])))
			out = append(out, buf[:n]...)
			i++
		case c >= 0xD800 && c <= 0xDFFF:
			out = append(out, 0xEF, 0xBF, 0xBD)
		default:
			n := utf8.EncodeRune(buf[:], c)
			out = append(out, buf[:n]...)
		}
	}
	return out
}

// String is UTF8 as a Go string.
func (s Str) String() string {
	return string(s.UTF8())
}

// CharCodeAt is str.charCodeAt(i): the code unit at i, or 0 when i is outside the string. The
// original's own helpers read a code point past the end as 0, so an out of range read has the
// same meaning here.
func (s Str) CharCodeAt(i int) uint16 {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}

// Substring is str.substring(start, end) with both ends clamped into the string.
func (s Str) Substring(start, end int) Str {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	if start > len(s) {
		start = len(s)
	}
	if end < start {
		start, end = end, start
	}
	out := make(Str, end-start)
	copy(out, s[start:end])
	return out
}

// Slice is str.slice(start) from start to the end.
func (s Str) Slice(start int) Str {
	return s.Substring(start, len(s))
}

// IndexOfFrom is str.indexOf(needle, from): the code unit index of the first occurrence at or
// after from, or -1.
func (s Str) IndexOfFrom(needle Str, from int) int {
	if from < 0 {
		from = 0
	}
	if len(needle) == 0 {
		if from > len(s) {
			return len(s)
		}
		return from
	}
	for i := from; i+len(needle) <= len(s); i++ {
		match := true
		for j := range needle {
			if s[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// Equal reports whether two strings hold the same code units.
func (s Str) Equal(other Str) bool {
	if len(s) != len(other) {
		return false
	}
	for i := range s {
		if s[i] != other[i] {
			return false
		}
	}
	return true
}

// EqualString compares against an ASCII literal.
func (s Str) EqualString(other string) bool {
	return s.Equal(FromString(other))
}

// Concat joins strings.
func Concat(parts ...Str) Str {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make(Str, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// ToLower is str.toLowerCase(): the Unicode simple lower case of every code point, with the one
// full mapping JavaScript applies that is not a single code point (U+0130 becomes U+0069 U+0307).
func (s Str) ToLower() Str {
	out := make(Str, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := rune(s[i])
		if c >= 0xD800 && c <= 0xDBFF && i+1 < len(s) && s[i+1] >= 0xDC00 && s[i+1] <= 0xDFFF {
			r := unicode.ToLower(utf16.DecodeRune(c, rune(s[i+1])))
			r1, r2 := utf16.EncodeRune(r)
			out = append(out, uint16(r1), uint16(r2))
			i++
			continue
		}
		if c == 0x0130 {
			out = append(out, 0x0069, 0x0307)
			continue
		}
		out = append(out, uint16(unicode.ToLower(c)))
	}
	return out
}

// ToLowerASCII lower cases the ASCII letters only, which is what the original's own string
// comparison helpers do while reading CSS text.
func ToLowerASCII(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}, s)
}
