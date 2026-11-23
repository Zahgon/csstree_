package tokenizer

// The character predicates of https://drafts.csswg.org/css-syntax-3/ § 4.2. Definitions. A code of
// 0 means EOF, the value the scanning helpers read past the end of the source.

const eofCode = 0

// IsDigit reports whether code is a digit: a code point between U+0030 DIGIT ZERO (0) and
// U+0039 DIGIT NINE (9).
func IsDigit(code uint16) bool {
	return code >= 0x0030 && code <= 0x0039
}

// IsHexDigit reports whether code is a hex digit: a digit, or a code point between U+0041 LATIN
// CAPITAL LETTER A (A) and U+0046 LATIN CAPITAL LETTER F (F), or a code point between U+0061 LATIN
// SMALL LETTER A (a) and U+0066 LATIN SMALL LETTER F (f).
func IsHexDigit(code uint16) bool {
	return IsDigit(code) || // 0 .. 9
		(code >= 0x0041 && code <= 0x0046) || // A .. F
		(code >= 0x0061 && code <= 0x0066) // a .. f
}

// IsUppercaseLetter reports whether code is a code point between U+0041 LATIN CAPITAL LETTER A (A)
// and U+005A LATIN CAPITAL LETTER Z (Z).
func IsUppercaseLetter(code uint16) bool {
	return code >= 0x0041 && code <= 0x005A
}

// IsLowercaseLetter reports whether code is a code point between U+0061 LATIN SMALL LETTER A (a)
// and U+007A LATIN SMALL LETTER Z (z).
func IsLowercaseLetter(code uint16) bool {
	return code >= 0x0061 && code <= 0x007A
}

// IsLetter reports whether code is an uppercase letter or a lowercase letter.
func IsLetter(code uint16) bool {
	return IsUppercaseLetter(code) || IsLowercaseLetter(code)
}

// IsNonAscii reports whether code is a code point with a value equal to or greater than
// U+0080 <control>.
//
// 2024-09-02: The latest spec narrows the range for non-ASCII characters (see
// https://github.com/csstree/csstree/issues/188). However, all modern browsers support a wider
// range, and strictly following the latest spec could result in some CSS being parsed incorrectly,
// even though it works in the browser. Therefore, this function adheres to the previous, broader
// definition of non-ASCII characters.
func IsNonAscii(code uint16) bool {
	return code >= 0x0080
}

// IsNameStart reports whether code is a name-start code point: a letter, a non-ASCII code point,
// or U+005F LOW LINE (_).
func IsNameStart(code uint16) bool {
	return IsLetter(code) || IsNonAscii(code) || code == 0x005F
}

// IsName reports whether code is a name code point: a name-start code point, a digit, or
// U+002D HYPHEN-MINUS (-).
func IsName(code uint16) bool {
	return IsNameStart(code) || IsDigit(code) || code == 0x002D
}

// IsNonPrintable reports whether code is a non-printable code point: a code point between
// U+0000 NULL and U+0008 BACKSPACE, or U+000B LINE TABULATION, or a code point between
// U+000E SHIFT OUT and U+001F INFORMATION SEPARATOR ONE, or U+007F DELETE.
func IsNonPrintable(code uint16) bool {
	return (code <= 0x0008) ||
		(code == 0x000B) ||
		(code >= 0x000E && code <= 0x001F) ||
		(code == 0x007F)
}

// IsNewline reports whether code is a newline. The specification lists U+000A LINE FEED alone,
// because U+000D CARRIAGE RETURN and U+000C FORM FEED are converted to it during preprocessing;
// this implementation does no preprocessing, so it checks for all three.
func IsNewline(code uint16) bool {
	return code == 0x000A || code == 0x000D || code == 0x000C
}

// IsWhiteSpace reports whether code is whitespace: a newline, U+0009 CHARACTER TABULATION, or
// U+0020 SPACE.
func IsWhiteSpace(code uint16) bool {
	return IsNewline(code) || code == 0x0020 || code == 0x0009
}

// IsValidEscape checks if two code points are a valid escape (§ 4.3.8).
func IsValidEscape(first, second uint16) bool {
	// If the first code point is not U+005C REVERSE SOLIDUS (\), return false.
	if first != 0x005C {
		return false
	}

	// Otherwise, if the second code point is a newline or EOF, return false.
	if IsNewline(second) || second == eofCode {
		return false
	}

	// Otherwise, return true.
	return true
}

// IsIdentifierStart checks if three code points would start an identifier (§ 4.3.9).
func IsIdentifierStart(first, second, third uint16) bool {
	// Look at the first code point:

	// U+002D HYPHEN-MINUS
	if first == 0x002D {
		// If the second code point is a name-start code point or a U+002D HYPHEN-MINUS,
		// or the second and third code points are a valid escape, return true. Otherwise,
		// return false.
		return IsNameStart(second) ||
			second == 0x002D ||
			IsValidEscape(second, third)
	}

	// name-start code point
	if IsNameStart(first) {
		// Return true.
		return true
	}

	// U+005C REVERSE SOLIDUS (\)
	if first == 0x005C {
		// If the first and second code points are a valid escape, return true. Otherwise,
		// return false.
		return IsValidEscape(first, second)
	}

	// anything else
	// Return false.
	return false
}

// IsNumberStart checks if three code points would start a number (§ 4.3.10).
func IsNumberStart(first, second, third uint16) bool {
	// Look at the first code point:

	// U+002B PLUS SIGN (+)
	// U+002D HYPHEN-MINUS (-)
	if first == 0x002B || first == 0x002D {
		// If the second code point is a digit, return true.
		if IsDigit(second) {
			return true
		}

		// Otherwise, if the second code point is a U+002E FULL STOP (.)
		// and the third code point is a digit, return true.
		// Otherwise, return false.
		return second == 0x002E && IsDigit(third)
	}

	// U+002E FULL STOP (.)
	if first == 0x002E {
		// If the second code point is a digit, return true. Otherwise, return false.
		return IsDigit(second)
	}

	// digit
	if IsDigit(first) {
		// Return true.
		return true
	}

	// anything else
	// Return false.
	return false
}

// IsBOM detects a byte order mark (https://en.wikipedia.org/wiki/Byte_order_mark) and returns the
// number of code units to skip.
func IsBOM(code uint16) int {
	// UTF-16BE
	if code == 0xFEFF {
		return 1
	}

	// UTF-16LE
	if code == 0xFFFE {
		return 1
	}

	return 0
}

// The fast code categories. Only ASCII code points have a special meaning, which is why the map
// below covers the codes 0..127 only.
const (
	EofCategory          = 0x80
	WhiteSpaceCategory   = 0x82
	DigitCategory        = 0x83
	NameStartCategory    = 0x84
	NonPrintableCategory = 0x85
)

var category = buildCategory()

func buildCategory() [0x80]int {
	var table [0x80]int

	for i := range table {
		code := uint16(i)

		switch {
		case IsWhiteSpace(code):
			table[i] = WhiteSpaceCategory
		case IsDigit(code):
			table[i] = DigitCategory
		case IsNameStart(code):
			table[i] = NameStartCategory
		case IsNonPrintable(code):
			table[i] = NonPrintableCategory
		case i != 0:
			table[i] = i
		default:
			table[i] = EofCategory
		}
	}

	return table
}

// CharCodeCategory returns the category of a code point. A code of 0 means EOF, but since
// U+0000 NULL is a non-printable code point the map gives it NonPrintableCategory, so
// EofCategory is never the result here.
func CharCodeCategory(code uint16) int {
	if code < 0x80 {
		return category[code]
	}

	return NameStartCategory
}
