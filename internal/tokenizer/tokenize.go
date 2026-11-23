package tokenizer

import "github.com/csstree/csstree/internal/jsutil"

var commentEnd = jsutil.FromString("*/")

// Tokenize splits source into tokens and calls onToken with the type, start offset and end offset
// of each one, following https://drafts.csswg.org/css-syntax-3/ § 4.3.1 Consume a token. No
// <EOF-token> is reported: the walk simply ends at the end of the source.
func Tokenize(source jsutil.Str, onToken func(tokenType, start, end int)) {
	sourceLength := len(source)

	getCharCode := func(offset int) uint16 {
		if offset < sourceLength {
			return source.CharCodeAt(offset)
		}
		return 0
	}

	start := IsBOM(getCharCode(0))
	offset := start
	var tokenType int

	var consumeUrlToken func()

	// § 4.3.3. Consume a numeric token
	consumeNumericToken := func() {
		// Consume a number and let number be the result.
		offset = ConsumeNumber(source, offset)

		// If the next 3 input code points would start an identifier, then:
		if IsIdentifierStart(getCharCode(offset), getCharCode(offset+1), getCharCode(offset+2)) {
			// Create a <dimension-token> with the same value and type flag as number,
			// and a unit set initially to the empty string.
			// Consume a name. Set the <dimension-token>'s unit to the returned value.
			// Return the <dimension-token>.
			tokenType = Dimension
			offset = ConsumeName(source, offset)
			return
		}

		// Otherwise, if the next input code point is U+0025 PERCENTAGE SIGN (%), consume it.
		if getCharCode(offset) == 0x0025 {
			// Create a <percentage-token> with the same value as number, and return it.
			tokenType = Percentage
			offset++
			return
		}

		// Otherwise, create a <number-token> with the same value and type flag as number,
		// and return it.
		tokenType = Number
	}

	// § 4.3.4. Consume an ident-like token
	consumeIdentLikeToken := func() {
		nameStartOffset := offset

		// Consume a name, and let string be the result.
		offset = ConsumeName(source, offset)

		// If string's value is an ASCII case-insensitive match for "url",
		// and the next input code point is U+0028 LEFT PARENTHESIS ((), consume it.
		if CmpStr(source, nameStartOffset, offset, "url") && getCharCode(offset) == 0x0028 {
			// While the next two input code points are whitespace, consume the next input
			// code point.
			offset = FindWhiteSpaceEnd(source, offset+1)

			// If the next one or two input code points are U+0022 QUOTATION MARK ("),
			// U+0027 APOSTROPHE ('), or whitespace followed by U+0022 QUOTATION MARK (")
			// or U+0027 APOSTROPHE ('), then create a <function-token> with its value set
			// to string and return it.
			if getCharCode(offset) == 0x0022 ||
				getCharCode(offset) == 0x0027 {
				tokenType = Function
				offset = nameStartOffset + 4
				return
			}

			// Otherwise, consume a url token, and return it.
			consumeUrlToken()
			return
		}

		// Otherwise, if the next input code point is U+0028 LEFT PARENTHESIS ((), consume
		// it. Create a <function-token> with its value set to string and return it.
		if getCharCode(offset) == 0x0028 {
			tokenType = Function
			offset++
			return
		}

		// Otherwise, create an <ident-token> with its value set to string and return it.
		tokenType = Ident
	}

	// § 4.3.5. Consume a string token
	//
	// The original is called without an ending code point everywhere, so the one it reads at
	// the current offset is always U+0022 QUOTATION MARK (") or U+0027 APOSTROPHE (').
	consumeStringToken := func(endingCodePoint uint16) {
		// This algorithm may be called with an ending code point, which denotes the code
		// point that ends the string. If an ending code point is not specified,
		// the current input code point is used.
		if endingCodePoint == 0 {
			endingCodePoint = getCharCode(offset)
			offset++
		}

		// Initially create a <string-token> with its value set to the empty string.
		tokenType = String

		// Repeatedly consume the next input code point from the stream:
		for ; offset < len(source); offset++ {
			code := source.CharCodeAt(offset)

			// The ending code point is compared against the category of the code point,
			// which for an ASCII code point that is none of the special categories is
			// the code point itself.
			switch CharCodeCategory(code) {
			// ending code point
			case int(endingCodePoint):
				// Return the <string-token>.
				offset++
				return

			// The end of the input ends the string as well, which the loop condition takes care
			// of; the specification calls that a parse error and returns the token all the same.

			// newline
			case WhiteSpaceCategory:
				if IsNewline(code) {
					// This is a parse error. Reconsume the current input code
					// point, create a <bad-string-token>, and return it.
					offset += GetNewlineLength(source, offset, code)
					tokenType = BadString
					return
				}

			// U+005C REVERSE SOLIDUS (\)
			case 0x005C:
				// If the next input code point is EOF, do nothing.
				if offset == len(source)-1 {
					break
				}

				nextCode := getCharCode(offset + 1)

				// Otherwise, if the next input code point is a newline, consume it.
				if IsNewline(nextCode) {
					offset += GetNewlineLength(source, offset+1, nextCode)
				} else if IsValidEscape(code, nextCode) {
					// Otherwise, (the stream starts with a valid escape) consume
					// an escaped code point and append the returned code point to
					// the <string-token>'s value.
					offset = ConsumeEscaped(source, offset) - 1
				}

				// anything else
				// Append the current input code point to the <string-token>'s value.
			}
		}
	}

	// § 4.3.6. Consume a url token
	// Note: This algorithm assumes that the initial "url(" has already been consumed.
	// This algorithm also assumes that it's being called to consume an "unquoted" value, like
	// url(foo). A quoted value, like url("foo"), is parsed as a <function-token>. Consume an
	// ident-like token automatically handles this distinction; this algorithm shouldn't be
	// called directly otherwise.
	consumeUrlToken = func() {
		// Initially create a <url-token> with its value set to the empty string.
		tokenType = Url

		// Consume as much whitespace as possible.
		offset = FindWhiteSpaceEnd(source, offset)

		// Repeatedly consume the next input code point from the stream:
		for ; offset < len(source); offset++ {
			code := source.CharCodeAt(offset)

			switch CharCodeCategory(code) {
			// U+0029 RIGHT PARENTHESIS ())
			case 0x0029:
				// Return the <url-token>.
				offset++
				return

			// The end of the input ends the url as well, which the loop condition takes care of;
			// the specification calls that a parse error and returns the token all the same.

			// whitespace
			case WhiteSpaceCategory:
				// Consume as much whitespace as possible.
				offset = FindWhiteSpaceEnd(source, offset)

				// If the next input code point is U+0029 RIGHT PARENTHESIS ()) or
				// EOF, consume it and return the <url-token>
				// (if EOF was encountered, this is a parse error);
				if getCharCode(offset) == 0x0029 || offset >= len(source) {
					if offset < len(source) {
						offset++
					}
					return
				}

				// otherwise, consume the remnants of a bad url, create a
				// <bad-url-token>, and return it.
				offset = ConsumeBadUrlRemnants(source, offset)
				tokenType = BadUrl
				return

			// U+0022 QUOTATION MARK (")
			// U+0027 APOSTROPHE (')
			// U+0028 LEFT PARENTHESIS (()
			// non-printable code point
			case 0x0022, 0x0027, 0x0028, NonPrintableCategory:
				// This is a parse error. Consume the remnants of a bad url,
				// create a <bad-url-token>, and return it.
				offset = ConsumeBadUrlRemnants(source, offset)
				tokenType = BadUrl
				return

			// U+005C REVERSE SOLIDUS (\)
			case 0x005C:
				// If the stream starts with a valid escape, consume an escaped code
				// point and append the returned code point to the <url-token>'s
				// value.
				if IsValidEscape(code, getCharCode(offset+1)) {
					offset = ConsumeEscaped(source, offset) - 1
					break
				}

				// Otherwise, this is a parse error. Consume the remnants of a bad
				// url, create a <bad-url-token>, and return it.
				offset = ConsumeBadUrlRemnants(source, offset)
				tokenType = BadUrl
				return

				// anything else
				// Append the current input code point to the <url-token>'s value.
			}
		}
	}

	// https://drafts.csswg.org/css-syntax-3/#consume-token
	// § 4.3.1. Consume a token
	for offset < sourceLength {
		code := source.CharCodeAt(offset)

		switch CharCodeCategory(code) {
		// whitespace
		case WhiteSpaceCategory:
			// Consume as much whitespace as possible. Return a <whitespace-token>.
			tokenType = WhiteSpace
			offset = FindWhiteSpaceEnd(source, offset+1)

		// U+0022 QUOTATION MARK (")
		case 0x0022:
			// Consume a string token and return it.
			consumeStringToken(0)

		// U+0023 NUMBER SIGN (#)
		case 0x0023:
			// If the next input code point is a name code point or the next two input
			// code points are a valid escape, then:
			if IsName(getCharCode(offset+1)) || IsValidEscape(getCharCode(offset+1), getCharCode(offset+2)) {
				// Create a <hash-token>.
				tokenType = Hash

				// If the next 3 input code points would start an identifier, set the
				// <hash-token>'s type flag to "id". The original leaves that flag
				// unset.

				// Consume a name, and set the <hash-token>'s value to the returned
				// string.
				offset = ConsumeName(source, offset+1)

				// Return the <hash-token>.
			} else {
				// Otherwise, return a <delim-token> with its value set to the
				// current input code point.
				tokenType = Delim
				offset++
			}

		// U+0027 APOSTROPHE (')
		case 0x0027:
			// Consume a string token and return it.
			consumeStringToken(0)

		// U+0028 LEFT PARENTHESIS (()
		case 0x0028:
			// Return a <(-token>.
			tokenType = LeftParenthesis
			offset++

		// U+0029 RIGHT PARENTHESIS ())
		case 0x0029:
			// Return a <)-token>.
			tokenType = RightParenthesis
			offset++

		// U+002B PLUS SIGN (+)
		case 0x002B:
			// If the input stream starts with a number, ...
			if IsNumberStart(code, getCharCode(offset+1), getCharCode(offset+2)) {
				// ... reconsume the current input code point, consume a numeric
				// token, and return it.
				consumeNumericToken()
			} else {
				// Otherwise, return a <delim-token> with its value set to the
				// current input code point.
				tokenType = Delim
				offset++
			}

		// U+002C COMMA (,)
		case 0x002C:
			// Return a <comma-token>.
			tokenType = Comma
			offset++

		// U+002D HYPHEN-MINUS (-)
		case 0x002D:
			// If the input stream starts with a number, reconsume the current input code
			// point, consume a numeric token, and return it.
			if IsNumberStart(code, getCharCode(offset+1), getCharCode(offset+2)) {
				consumeNumericToken()
			} else {
				// Otherwise, if the next 2 input code points are U+002D HYPHEN-MINUS
				// U+003E GREATER-THAN SIGN (->), consume them and return a
				// <CDC-token>.
				if getCharCode(offset+1) == 0x002D &&
					getCharCode(offset+2) == 0x003E {
					tokenType = CDC
					offset = offset + 3
				} else {
					// Otherwise, if the input stream starts with an identifier,
					// ...
					if IsIdentifierStart(code, getCharCode(offset+1), getCharCode(offset+2)) {
						// ... reconsume the current input code point,
						// consume an ident-like token, and return it.
						consumeIdentLikeToken()
					} else {
						// Otherwise, return a <delim-token> with its value
						// set to the current input code point.
						tokenType = Delim
						offset++
					}
				}
			}

		// U+002E FULL STOP (.)
		case 0x002E:
			// If the input stream starts with a number, ...
			if IsNumberStart(code, getCharCode(offset+1), getCharCode(offset+2)) {
				// ... reconsume the current input code point, consume a numeric
				// token, and return it.
				consumeNumericToken()
			} else {
				// Otherwise, return a <delim-token> with its value set to the
				// current input code point.
				tokenType = Delim
				offset++
			}

		// U+002F SOLIDUS (/)
		case 0x002F:
			// If the next two input code point are U+002F SOLIDUS (/) followed by a
			// U+002A ASTERISK (*),
			if getCharCode(offset+1) == 0x002A {
				// ... consume them and all following code points up to and including
				// the first U+002A ASTERISK (*) followed by a U+002F SOLIDUS (/), or
				// up to an EOF code point.
				tokenType = Comment
				offset = source.IndexOfFrom(commentEnd, offset+2)
				if offset == -1 {
					offset = len(source)
				} else {
					offset = offset + 2
				}
			} else {
				tokenType = Delim
				offset++
			}

		// U+003A COLON (:)
		case 0x003A:
			// Return a <colon-token>.
			tokenType = Colon
			offset++

		// U+003B SEMICOLON (;)
		case 0x003B:
			// Return a <semicolon-token>.
			tokenType = Semicolon
			offset++

		// U+003C LESS-THAN SIGN (<)
		case 0x003C:
			// If the next 3 input code points are U+0021 EXCLAMATION MARK U+002D
			// HYPHEN-MINUS U+002D HYPHEN-MINUS (!--), ...
			if getCharCode(offset+1) == 0x0021 &&
				getCharCode(offset+2) == 0x002D &&
				getCharCode(offset+3) == 0x002D {
				// ... consume them and return a <CDO-token>.
				tokenType = CDO
				offset = offset + 4
			} else {
				// Otherwise, return a <delim-token> with its value set to the
				// current input code point.
				tokenType = Delim
				offset++
			}

		// U+0040 COMMERCIAL AT (@)
		case 0x0040:
			// If the next 3 input code points would start an identifier, ...
			if IsIdentifierStart(getCharCode(offset+1), getCharCode(offset+2), getCharCode(offset+3)) {
				// ... consume a name, create an <at-keyword-token> with its value set
				// to the returned value, and return it.
				tokenType = AtKeyword
				offset = ConsumeName(source, offset+1)
			} else {
				// Otherwise, return a <delim-token> with its value set to the
				// current input code point.
				tokenType = Delim
				offset++
			}

		// U+005B LEFT SQUARE BRACKET ([)
		case 0x005B:
			// Return a <[-token>.
			tokenType = LeftSquareBracket
			offset++

		// U+005C REVERSE SOLIDUS (\)
		case 0x005C:
			// If the input stream starts with a valid escape, ...
			if IsValidEscape(code, getCharCode(offset+1)) {
				// ... reconsume the current input code point, consume an ident-like
				// token, and return it.
				consumeIdentLikeToken()
			} else {
				// Otherwise, this is a parse error. Return a <delim-token> with its
				// value set to the current input code point.
				tokenType = Delim
				offset++
			}

		// U+005D RIGHT SQUARE BRACKET (])
		case 0x005D:
			// Return a <]-token>.
			tokenType = RightSquareBracket
			offset++

		// U+007B LEFT CURLY BRACKET ({)
		case 0x007B:
			// Return a <{-token>.
			tokenType = LeftCurlyBracket
			offset++

		// U+007D RIGHT CURLY BRACKET (})
		case 0x007D:
			// Return a <}-token>.
			tokenType = RightCurlyBracket
			offset++

		// digit
		case DigitCategory:
			// Reconsume the current input code point, consume a numeric token, and
			// return it.
			consumeNumericToken()

		// name-start code point
		case NameStartCategory:
			// Reconsume the current input code point, consume an ident-like token, and
			// return it.
			consumeIdentLikeToken()

		// The end of the input would return an <EOF-token>; the loop ends there instead, and the
		// token stream adds the one <EOF-token> itself.

		// anything else
		default:
			// Return a <delim-token> with its value set to the current input code point.
			tokenType = Delim
			offset++
		}

		// put token to stream
		onToken(tokenType, start, offset)
		start = offset
	}
}
