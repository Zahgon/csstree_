// Package cssutil carries the string, url and identifier encoding and decoding of CSS text and
// the descriptors a keyword or a property name is read into.
package cssutil

import (
	"strconv"

	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

const (
	reverseSolidus   = 0x005C // \
	quotationMark    = 0x0022 // "
	apostrophe       = 0x0027 // '
	space            = 0x0020
	leftParenthesis  = 0x0028 // (
	rightParenthesis = 0x0029 // )
	hyphenMinus      = 0x002D // -
	carriageReturn   = 0x000D
	lineFeed         = 0x000A
	replacementChar  = 0xFFFD
)

// decodeEscapes walks a run of CSS text between start and end, turning every escape into the code
// point it names. It is the body the three decoders share, differing only in where they begin and
// end and in what they do with a backslash at the very end.
func decodeEscapes(str jsutil.Str, start, end int, keepTrailing bool) jsutil.Str {
	length := len(str)
	decoded := jsutil.Str{}

	for i := start; i <= end; i++ {
		code := str.CharCodeAt(i)

		if code == reverseSolidus {
			// special case at the ending
			if i == end {
				// if the next input code point is EOF, do nothing
				// otherwise include the last delimiter as escaped
				if keepTrailing && i != length-1 {
					decoded = str.Slice(i + 1)
				}
				break
			}

			i++
			code = str.CharCodeAt(i)

			// consume escaped
			if tokenizer.IsValidEscape(reverseSolidus, code) {
				escapeStart := i - 1
				escapeEnd := tokenizer.ConsumeEscaped(str, escapeStart)

				i = escapeEnd - 1
				decoded = append(decoded, tokenizer.DecodeEscaped(str.Substring(escapeStart+1, escapeEnd))...)
			} else if code == carriageReturn && str.CharCodeAt(i+1) == lineFeed {
				// \r\n
				i++
			}
		} else {
			decoded = append(decoded, str[i])
		}
	}

	return decoded
}

// DecodeString reads the text a quoted string stands for.
func DecodeString(str jsutil.Str) jsutil.Str {
	length := len(str)
	firstChar := str.CharCodeAt(0)
	start := 0
	if firstChar == quotationMark || firstChar == apostrophe {
		start = 1
	}
	end := length - 1
	if start == 1 && length > 1 && str.CharCodeAt(length-1) == firstChar {
		end = length - 2
	}
	return decodeEscapes(str, start, end, true)
}

// EncodeString writes a string the way § 2.1 of CSSOM serializes one.
// https://drafts.csswg.org/cssom/#serialize-a-string
func EncodeString(str jsutil.Str, useApostrophe bool) jsutil.Str {
	quote := uint16(quotationMark)
	if useApostrophe {
		quote = apostrophe
	}
	encoded := jsutil.Str{quote}
	wsBeforeHexIsNeeded := false

	for i := 0; i < len(str); i++ {
		code := str[i]

		// If the character is NULL (U+0000), then the REPLACEMENT CHARACTER (U+FFFD).
		if code == 0x0000 {
			encoded = append(encoded, replacementChar)
			continue
		}

		// If the character is in the range [\1-\1f] (U+0001 to U+001F) or is U+007F,
		// the character escaped as code point.
		if code <= 0x001F || code == 0x007F {
			encoded = append(encoded, reverseSolidus)
			encoded = append(encoded, jsutil.FromString(strconv.FormatInt(int64(code), 16))...)
			wsBeforeHexIsNeeded = true
			continue
		}

		// If the character is the quote or "\" (U+005C), the escaped character.
		if code == quote || code == reverseSolidus {
			encoded = append(encoded, reverseSolidus, code)
			wsBeforeHexIsNeeded = false
		} else {
			if wsBeforeHexIsNeeded && (tokenizer.IsHexDigit(code) || tokenizer.IsWhiteSpace(code)) {
				encoded = append(encoded, space)
			}

			encoded = append(encoded, code)
			wsBeforeHexIsNeeded = false
		}
	}

	return append(encoded, quote)
}

// DecodeIdent reads the name an identifier stands for.
func DecodeIdent(str jsutil.Str) jsutil.Str {
	return decodeEscapes(str, 0, len(str)-1, false)
}

// EncodeIdent writes an identifier the way § 2.1 of CSSOM serializes one.
// https://drafts.csswg.org/cssom/#serialize-an-identifier
func EncodeIdent(str jsutil.Str) jsutil.Str {
	// If the character is the first character and is a "-" (U+002D), and there is no second
	// character, then the escaped character.
	if len(str) == 1 && str[0] == hyphenMinus {
		return jsutil.FromString("\\-")
	}

	encoded := jsutil.Str{}

	for i := 0; i < len(str); i++ {
		code := str[i]

		// If the character is NULL (U+0000), then the REPLACEMENT CHARACTER (U+FFFD).
		if code == 0x0000 {
			encoded = append(encoded, replacementChar)
			continue
		}

		if code <= 0x001F || code == 0x007F ||
			(code >= 0x0030 && code <= 0x0039 && (i == 0 || (i == 1 && str[0] == hyphenMinus))) {
			// ... then the character escaped as code point.
			encoded = append(encoded, reverseSolidus)
			encoded = append(encoded, jsutil.FromString(strconv.FormatInt(int64(code), 16))...)
			encoded = append(encoded, space)
			continue
		}

		if tokenizer.IsName(code) {
			encoded = append(encoded, code)
		} else {
			encoded = append(encoded, reverseSolidus, code)
		}
	}

	return encoded
}

// DecodeURL reads the address a url token stands for.
func DecodeURL(str jsutil.Str) jsutil.Str {
	length := len(str)
	start := 4 // length of "url("
	end := length - 1
	if str.CharCodeAt(length-1) == rightParenthesis {
		end = length - 2
	}

	for start < end && tokenizer.IsWhiteSpace(str.CharCodeAt(start)) {
		start++
	}

	for start < end && tokenizer.IsWhiteSpace(str.CharCodeAt(end)) {
		end--
	}

	return decodeEscapes(str, start, end, true)
}

// EncodeURL writes a url token for an address.
func EncodeURL(str jsutil.Str) jsutil.Str {
	encoded := jsutil.FromString("url(")
	wsBeforeHexIsNeeded := false

	for i := 0; i < len(str); i++ {
		code := str[i]

		if code == 0x0000 {
			encoded = append(encoded, replacementChar)
			continue
		}

		if code <= 0x001F || code == 0x007F {
			encoded = append(encoded, reverseSolidus)
			encoded = append(encoded, jsutil.FromString(strconv.FormatInt(int64(code), 16))...)
			wsBeforeHexIsNeeded = true
			continue
		}

		if code == space || code == reverseSolidus || code == quotationMark ||
			code == apostrophe || code == leftParenthesis || code == rightParenthesis {
			encoded = append(encoded, reverseSolidus, code)
			wsBeforeHexIsNeeded = false
		} else {
			if wsBeforeHexIsNeeded && tokenizer.IsHexDigit(code) {
				encoded = append(encoded, space)
			}

			encoded = append(encoded, code)
			wsBeforeHexIsNeeded = false
		}
	}

	return append(encoded, rightParenthesis)
}
