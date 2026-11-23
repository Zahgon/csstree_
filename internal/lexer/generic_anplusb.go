package lexer

import (
	"github.com/csstree/csstree/internal/generator"
	"github.com/csstree/csstree/internal/tokenizer"
)

const (
	plusSignCode     = 0x002B // +
	hyphenMinusCode  = 0x002D // -
	letterNCode      = 0x006E // n
	questionMarkCode = 0x003F // ?
	letterUCode      = 0x0075 // u
)

func isDelimToken(token *generator.Token, code uint16) bool {
	return token != nil && token.Type == tokenizer.Delim && token.Value.CharCodeAt(0) == code
}

// skipSC walks past the white space and the comments from the offset on.
func skipSC(token *generator.Token, offset int, getNextToken func(int) *generator.Token) int {
	for token != nil && (token.Type == tokenizer.WhiteSpace || token.Type == tokenizer.Comment) {
		offset++
		token = getNextToken(offset)
	}

	return offset
}

// checkIntegerToken reads the rest of a token's text as an integer and answers the offset after it.
func checkIntegerToken(token *generator.Token, valueOffset int, disallowSign bool, offset int) float64 {
	if token == nil {
		return 0
	}

	code := token.Value.CharCodeAt(valueOffset)

	if code == plusSignCode || code == hyphenMinusCode {
		if disallowSign {
			// Number sign is not allowed
			return 0
		}
		valueOffset++
	}

	for ; valueOffset < len(token.Value); valueOffset++ {
		if !tokenizer.IsDigit(token.Value.CharCodeAt(valueOffset)) {
			// Integer is expected
			return 0
		}
	}

	return float64(offset + 1)
}

// consumeBToken reads the "+ b" part of an An+B value.
func consumeBToken(token *generator.Token, startOffset int, getNextToken func(int) *generator.Token) float64 {
	sign := false
	offset := skipSC(token, startOffset, getNextToken)

	token = getNextToken(offset)

	if token == nil {
		return float64(startOffset)
	}

	if token.Type != tokenizer.Number {
		if isDelimToken(token, plusSignCode) || isDelimToken(token, hyphenMinusCode) {
			sign = true
			offset++
			offset = skipSC(getNextToken(offset), offset, getNextToken)
			token = getNextToken(offset)

			if token == nil || token.Type != tokenizer.Number {
				return 0
			}
		} else {
			return float64(startOffset)
		}
	}

	if !sign {
		code := token.Value.CharCodeAt(0)
		if code != plusSignCode && code != hyphenMinusCode {
			// Number sign is expected
			return 0
		}
	}

	valueOffset := 1
	if sign {
		valueOffset = 0
	}

	return checkIntegerToken(token, valueOffset, sign, offset)
}

// anPlusB matches the An+B microsyntax.
// https://www.w3.org/TR/css-syntax-3/#anb
func anPlusB(token *generator.Token, getNextToken func(int) *generator.Token, _ any) float64 {
	offset := 0

	if token == nil {
		return 0
	}

	switch {
	// <integer>
	case token.Type == tokenizer.Number:
		return checkIntegerToken(token, 0, false, offset)

	// -n, -n <signed-integer>, -n ['+' | '-'] <signless-integer>, -n- <signless-integer>,
	// <dashndashdigit-ident>
	case token.Type == tokenizer.Ident && token.Value.CharCodeAt(0) == hyphenMinusCode:
		if !tokenizer.CmpChar(token.Value, 1, letterNCode) {
			return 0
		}

		switch len(token.Value) {
		case 2:
			offset++
			return consumeBToken(getNextToken(offset), offset, getNextToken)

		case 3:
			if token.Value.CharCodeAt(2) != hyphenMinusCode {
				return 0
			}

			offset++
			offset = skipSC(getNextToken(offset), offset, getNextToken)
			token = getNextToken(offset)

			return checkIntegerToken(token, 0, true, offset)

		default:
			if token.Value.CharCodeAt(2) != hyphenMinusCode {
				return 0
			}

			return checkIntegerToken(token, 3, true, offset)
		}

	// '+'? n and its forms
	case token.Type == tokenizer.Ident ||
		(isDelimToken(token, plusSignCode) && nextTokenType(getNextToken, offset+1) == tokenizer.Ident):
		// just ignore a plus
		if token.Type != tokenizer.Ident {
			offset++
			token = getNextToken(offset)
		}

		if token == nil || !tokenizer.CmpChar(token.Value, 0, letterNCode) {
			return 0
		}

		switch len(token.Value) {
		case 1:
			offset++
			return consumeBToken(getNextToken(offset), offset, getNextToken)

		case 2:
			if token.Value.CharCodeAt(1) != hyphenMinusCode {
				return 0
			}

			offset++
			offset = skipSC(getNextToken(offset), offset, getNextToken)
			token = getNextToken(offset)

			return checkIntegerToken(token, 0, true, offset)

		default:
			if token.Value.CharCodeAt(1) != hyphenMinusCode {
				return 0
			}

			return checkIntegerToken(token, 2, true, offset)
		}

	// <ndashdigit-dimension> and its forms
	case token.Type == tokenizer.Dimension:
		code := token.Value.CharCodeAt(0)
		sign := 0
		if code == plusSignCode || code == hyphenMinusCode {
			sign = 1
		}
		i := sign

		for ; i < len(token.Value); i++ {
			if !tokenizer.IsDigit(token.Value.CharCodeAt(i)) {
				break
			}
		}

		if i == sign {
			// Integer is expected
			return 0
		}

		if !tokenizer.CmpChar(token.Value, i, letterNCode) {
			return 0
		}

		if i+1 == len(token.Value) {
			offset++
			return consumeBToken(getNextToken(offset), offset, getNextToken)
		}

		if token.Value.CharCodeAt(i+1) != hyphenMinusCode {
			return 0
		}

		if i+2 == len(token.Value) {
			offset++
			offset = skipSC(getNextToken(offset), offset, getNextToken)
			token = getNextToken(offset)

			return checkIntegerToken(token, 0, true, offset)
		}

		return checkIntegerToken(token, i+2, true, offset)
	}

	return 0
}

// nextTokenType is the type of the token at the offset, or the end of the stream. The original
// reads the type of a token that may be absent, which fails there; here the absent token reads as
// the end of the input, which no branch matches.
func nextTokenType(getNextToken func(int) *generator.Token, offset int) int {
	if token := getNextToken(offset); token != nil {
		return token.Type
	}
	return tokenizer.EOF
}

// hexSequence reads the hexadecimal digits of a unicode range from the offset on.
func hexSequence(token *generator.Token, offset int, allowDash bool) int {
	hexlen := 0

	for pos := offset; pos < len(token.Value); pos++ {
		code := token.Value.CharCodeAt(pos)

		if code == hyphenMinusCode && allowDash && hexlen != 0 {
			hexSequence(token, offset+hexlen+1, false)
			return 6 // disallow following question marks
		}

		if !tokenizer.IsHexDigit(code) {
			return 0 // not a hex digit
		}

		hexlen++
		if hexlen > 6 {
			return 0 // too many hex digits
		}
	}

	return hexlen
}

// withQuestionMarkSequence takes the question marks that stand for any hex digit.
func withQuestionMarkSequence(consumed, length int, getNextToken func(int) *generator.Token) float64 {
	if consumed == 0 {
		return 0 // nothing consumed
	}

	for isDelimToken(getNextToken(length), questionMarkCode) {
		consumed++
		if consumed > 6 {
			return 0 // too many question marks
		}

		length++
	}

	return float64(length)
}

// urange matches a unicode range.
// https://drafts.csswg.org/css-syntax/#urange
func urange(token *generator.Token, getNextToken func(int) *generator.Token, _ any) float64 {
	length := 0

	// should start with `u` or `U`
	if token == nil || token.Type != tokenizer.Ident || !tokenizer.CmpChar(token.Value, 0, letterUCode) {
		return 0
	}

	length++
	token = getNextToken(length)
	if token == nil {
		return 0
	}

	// u '+' <ident-token> '?'*
	// u '+' '?'+
	if isDelimToken(token, plusSignCode) {
		length++
		token = getNextToken(length)
		if token == nil {
			return 0
		}

		if token.Type == tokenizer.Ident {
			length++
			return withQuestionMarkSequence(hexSequence(token, 0, true), length, getNextToken)
		}

		if isDelimToken(token, questionMarkCode) {
			length++
			return withQuestionMarkSequence(1, length, getNextToken)
		}

		// Hex digit or question mark is expected
		return 0
	}

	// u <number-token> '?'*
	// u <number-token> <dimension-token>
	// u <number-token> <number-token>
	if token.Type == tokenizer.Number {
		consumedHexLength := hexSequence(token, 1, true)
		if consumedHexLength == 0 {
			return 0
		}

		length++
		token = getNextToken(length)
		if token == nil {
			// u <number-token> <eof>
			return float64(length)
		}

		if token.Type == tokenizer.Dimension || token.Type == tokenizer.Number {
			if token.Value.CharCodeAt(0) != hyphenMinusCode || hexSequence(token, 1, false) == 0 {
				return 0
			}

			return float64(length + 1)
		}

		return withQuestionMarkSequence(consumedHexLength, length, getNextToken)
	}

	// u <dimension-token> '?'*
	if token.Type == tokenizer.Dimension {
		length++
		return withQuestionMarkSequence(hexSequence(token, 1, true), length, getNextToken)
	}

	return 0
}
