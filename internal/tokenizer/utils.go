package tokenizer

import (
	"math"
	"unicode/utf16"

	"github.com/csstree/csstree/internal/jsutil"
)

// GetNewlineLength returns the length of the newline at offset, where code is the code point read
// there: 2 for a U+000D CARRIAGE RETURN followed by a U+000A LINE FEED, 1 otherwise.
func GetNewlineLength(source jsutil.Str, offset int, code uint16) int {
	if code == 13 /* \r */ && source.CharCodeAt(offset+1) == 10 /* \n */ {
		return 2
	}

	return 1
}

// CmpChar reports whether the code point at offset matches referenceCode, with A..Z lower cased
// before the comparison.
func CmpChar(testStr jsutil.Str, offset int, referenceCode uint16) bool {
	code := testStr.CharCodeAt(offset)

	// code.toLowerCase() for A..Z
	if IsUppercaseLetter(code) {
		code = code | 32
	}

	return code == referenceCode
}

// CmpStr reports whether the code points of testStr between start and end match referenceStr, with
// A..Z lower cased before the comparison. referenceStr is an ASCII literal, as every caller of the
// original passes one.
func CmpStr(testStr jsutil.Str, start, end int, referenceStr string) bool {
	if end-start != len(referenceStr) {
		return false
	}

	if start < 0 || end > len(testStr) {
		return false
	}

	for i := start; i < end; i++ {
		referenceCode := uint16(referenceStr[i-start])
		testCode := testStr.CharCodeAt(i)

		// testCode.toLowerCase() for A..Z
		if IsUppercaseLetter(testCode) {
			testCode = testCode | 32
		}

		if testCode != referenceCode {
			return false
		}
	}

	return true
}

// FindWhiteSpaceStart walks back from offset over whitespace and returns the offset of the first
// whitespace code point of the run.
func FindWhiteSpaceStart(source jsutil.Str, offset int) int {
	for ; offset >= 0; offset-- {
		if !IsWhiteSpace(source.CharCodeAt(offset)) {
			break
		}
	}

	return offset + 1
}

// FindWhiteSpaceEnd walks forward from offset over whitespace and returns the offset just past the
// run.
func FindWhiteSpaceEnd(source jsutil.Str, offset int) int {
	for ; offset < len(source); offset++ {
		if !IsWhiteSpace(source.CharCodeAt(offset)) {
			break
		}
	}

	return offset
}

// FindDecimalNumberEnd walks forward from offset over digits and returns the offset just past
// them.
func FindDecimalNumberEnd(source jsutil.Str, offset int) int {
	for ; offset < len(source); offset++ {
		if !IsDigit(source.CharCodeAt(offset)) {
			break
		}
	}

	return offset
}

// ConsumeEscaped consumes an escaped code point (§ 4.3.7) and returns the offset just past it.
//
// It assumes that the U+005C REVERSE SOLIDUS (\) has already been consumed and that the next input
// code point has already been verified to be part of a valid escape.
func ConsumeEscaped(source jsutil.Str, offset int) int {
	offset += 2

	// hex digit
	if IsHexDigit(source.CharCodeAt(offset - 1)) {
		// Consume as many hex digits as possible, but no more than 5.
		// Note that this means 1-6 hex digits have been consumed in total.
		maxOffset := len(source)
		if offset+5 < maxOffset {
			maxOffset = offset + 5
		}
		for ; offset < maxOffset; offset++ {
			if !IsHexDigit(source.CharCodeAt(offset)) {
				break
			}
		}

		// If the next input code point is whitespace, consume it as well.
		code := source.CharCodeAt(offset)
		if IsWhiteSpace(code) {
			offset += GetNewlineLength(source, offset, code)
		}
	}

	return offset
}

// ConsumeName consumes a name (§ 4.3.11) and returns the offset just past it.
//
// Note: This algorithm does not do the verification of the first few code points that are
// necessary to ensure the returned code points would constitute an <ident-token>. If that is the
// intended use, ensure that the stream starts with an identifier before calling this algorithm.
func ConsumeName(source jsutil.Str, offset int) int {
	// Let result initially be an empty string.
	// Repeatedly consume the next input code point from the stream:
	for ; offset < len(source); offset++ {
		code := source.CharCodeAt(offset)

		// name code point
		if IsName(code) {
			// Append the code point to result.
			continue
		}

		// the stream starts with a valid escape
		if IsValidEscape(code, source.CharCodeAt(offset+1)) {
			// Consume an escaped code point. Append the returned code point to result.
			offset = ConsumeEscaped(source, offset) - 1
			continue
		}

		// anything else
		// Reconsume the current input code point. Return result.
		break
	}

	return offset
}

// ConsumeNumber consumes a number (§ 4.3.12) and returns the offset just past it.
func ConsumeNumber(source jsutil.Str, offset int) int {
	code := source.CharCodeAt(offset)

	// 2. If the next input code point is U+002B PLUS SIGN (+) or U+002D HYPHEN-MINUS (-),
	// consume it and append it to repr.
	if code == 0x002B || code == 0x002D {
		offset += 1
		code = source.CharCodeAt(offset)
	}

	// 3. While the next input code point is a digit, consume it and append it to repr.
	if IsDigit(code) {
		offset = FindDecimalNumberEnd(source, offset+1)
		code = source.CharCodeAt(offset)
	}

	// 4. If the next 2 input code points are U+002E FULL STOP (.) followed by a digit, then:
	if code == 0x002E && IsDigit(source.CharCodeAt(offset+1)) {
		// 4.1 Consume them.
		// 4.2 Append them to repr.
		offset += 2

		// 4.3 Set type to "number".

		// 4.4 While the next input code point is a digit, consume it and append it to repr.
		offset = FindDecimalNumberEnd(source, offset)
	}

	// 5. If the next 2 or 3 input code points are U+0045 LATIN CAPITAL LETTER E (E)
	// or U+0065 LATIN SMALL LETTER E (e), ... , followed by a digit, then:
	if CmpChar(source, offset, 101 /* e */) {
		sign := 0
		code = source.CharCodeAt(offset + 1)

		// ... optionally followed by U+002D HYPHEN-MINUS (-) or U+002B PLUS SIGN (+) ...
		if code == 0x002D || code == 0x002B {
			sign = 1
			code = source.CharCodeAt(offset + 2)
		}

		// ... followed by a digit
		if IsDigit(code) {
			// 5.1 Consume them.
			// 5.2 Append them to repr.

			// 5.3 Set type to "number".

			// 5.4 While the next input code point is a digit, consume it and append it to
			// repr.
			offset = FindDecimalNumberEnd(source, offset+1+sign+1)
		}
	}

	return offset
}

// ConsumeBadUrlRemnants consumes the remnants of a bad url (§ 4.3.14) and returns the offset it
// stopped at.
//
// Its sole use is to consume enough of the input stream to reach a recovery point where normal
// tokenizing can resume.
func ConsumeBadUrlRemnants(source jsutil.Str, offset int) int {
	// Repeatedly consume the next input code point from the stream:
	for ; offset < len(source); offset++ {
		code := source.CharCodeAt(offset)

		// U+0029 RIGHT PARENTHESIS ())
		// EOF
		if code == 0x0029 {
			// Return.
			offset++
			break
		}

		if IsValidEscape(code, source.CharCodeAt(offset+1)) {
			// Consume an escaped code point.
			// Note: This allows an escaped right parenthesis ("\)") to be encountered
			// without ending the <bad-url-token>. This is otherwise identical to
			// the "anything else" clause.
			offset = ConsumeEscaped(source, offset)
		}
	}

	return offset
}

// DecodeEscaped turns the body of an escape sequence into the code point it stands for (§ 4.3.7).
//
// Note: This algorithm assumes that escaped is valid without leading U+005C REVERSE SOLIDUS (\).
func DecodeEscaped(escaped jsutil.Str) jsutil.Str {
	// Single char escaped that's not a hex digit. The result is the first code unit, so an
	// escaped code point outside the basic plane keeps its leading surrogate only, as in the
	// original.
	if len(escaped) == 1 && !IsHexDigit(escaped.CharCodeAt(0)) {
		return escaped.Substring(0, 1)
	}

	// Interpret the hex digits as a hexadecimal number.
	code := jsutil.ParseIntRadix(escaped, 16)

	if code == 0 || // If this number is zero,
		(code >= 0xD800 && code <= 0xDFFF) || // or is for a surrogate,
		code > 0x10FFFF { // or is greater than the maximum allowed code point
		// ... return U+FFFD REPLACEMENT CHARACTER
		code = 0xFFFD
	}

	// Otherwise, return the code point with that value.
	return fromCodePoint(code)
}

// fromCodePoint is String.fromCodePoint for the values DecodeEscaped computes. The original throws
// a RangeError for a value that is not a code point; its callers only ever reach this with the
// value of one to six hex digits, so such a value stands in as U+FFFD here rather than ending the
// parse.
func fromCodePoint(code float64) jsutil.Str {
	if math.IsNaN(code) || code < 0 || code > 0x10FFFF {
		return jsutil.Str{0xFFFD}
	}

	if code <= 0xFFFF {
		return jsutil.Str{uint16(code)}
	}

	lead, trail := utf16.EncodeRune(rune(int32(code)))

	return jsutil.Str{uint16(lead), uint16(trail)}
}
