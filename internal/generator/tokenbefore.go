package generator

import (
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

const (
	plusSign    = 0x002B // U+002B PLUS SIGN (+)
	hyphenMinus = 0x002D // U+002D HYPHEN-MINUS (-)
)

// tokenRef is one side of a pair in the whitespace tables: either a token type, or the character a
// delim token starts with.
type tokenRef struct {
	tokenType int
	char      uint16
	isChar    bool
}

// typeRef names a token type.
func typeRef(tokenType int) tokenRef {
	return tokenRef{tokenType: tokenType}
}

// charRef names a delim by its character.
func charRef(char uint16) tokenRef {
	return tokenRef{char: char, isChar: true}
}

// code is the number a side of a pair is keyed by:
//
//	0xxxxxxx x0000000 - char code (0x80 for non-ASCII) or delim value
//	00000000 0xxxxxx0 - token type (0 for delim, 1 for token)
//	00000000 0000000x - reserved for carriage emit flag (0 for no space, 1 for space)
func (r tokenRef) code() int {
	if r.isChar {
		char := int(r.char)
		if char > 0x80 {
			char = 0x80 // replace non-ASCII with 0x80
		}
		return char << 7
	}
	return r.tokenType << 1
}

// tokenCode is the same number for a token about to be written. A delim is keyed by the character
// it starts with rather than by its type.
func tokenCode(tokenType int, value jsutil.Str) int {
	if tokenType == tokenizer.Delim {
		char := int(value.CharCodeAt(0))
		if char > 0x80 {
			char = 0x80 // replace non-ASCII with 0x80
		}
		return char << 7
	}
	return tokenType << 1
}

// pair is two tokens that need a whitespace between them.
type pair struct {
	prev tokenRef
	next tokenRef
}

// https://www.w3.org/TR/css-syntax-3/#serialization
// The only requirement for serialization is that it must "round-trip" with parsing,
// that is, parsing the stylesheet must produce the same data structures as parsing,
// serializing, and parsing again, except for consecutive <whitespace-token>s,
// which may be collapsed into a single token.
var specPairs = []pair{
	{typeRef(tokenizer.Ident), typeRef(tokenizer.Ident)},
	{typeRef(tokenizer.Ident), typeRef(tokenizer.Function)},
	{typeRef(tokenizer.Ident), typeRef(tokenizer.Url)},
	{typeRef(tokenizer.Ident), typeRef(tokenizer.BadUrl)},
	{typeRef(tokenizer.Ident), charRef('-')},
	{typeRef(tokenizer.Ident), typeRef(tokenizer.Number)},
	{typeRef(tokenizer.Ident), typeRef(tokenizer.Percentage)},
	{typeRef(tokenizer.Ident), typeRef(tokenizer.Dimension)},
	{typeRef(tokenizer.Ident), typeRef(tokenizer.CDC)},
	{typeRef(tokenizer.Ident), typeRef(tokenizer.LeftParenthesis)},

	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.Ident)},
	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.Function)},
	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.Url)},
	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.BadUrl)},
	{typeRef(tokenizer.AtKeyword), charRef('-')},
	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.Number)},
	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.Percentage)},
	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.Dimension)},
	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.CDC)},

	{typeRef(tokenizer.Hash), typeRef(tokenizer.Ident)},
	{typeRef(tokenizer.Hash), typeRef(tokenizer.Function)},
	{typeRef(tokenizer.Hash), typeRef(tokenizer.Url)},
	{typeRef(tokenizer.Hash), typeRef(tokenizer.BadUrl)},
	{typeRef(tokenizer.Hash), charRef('-')},
	{typeRef(tokenizer.Hash), typeRef(tokenizer.Number)},
	{typeRef(tokenizer.Hash), typeRef(tokenizer.Percentage)},
	{typeRef(tokenizer.Hash), typeRef(tokenizer.Dimension)},
	{typeRef(tokenizer.Hash), typeRef(tokenizer.CDC)},

	{typeRef(tokenizer.Dimension), typeRef(tokenizer.Ident)},
	{typeRef(tokenizer.Dimension), typeRef(tokenizer.Function)},
	{typeRef(tokenizer.Dimension), typeRef(tokenizer.Url)},
	{typeRef(tokenizer.Dimension), typeRef(tokenizer.BadUrl)},
	{typeRef(tokenizer.Dimension), charRef('-')},
	{typeRef(tokenizer.Dimension), typeRef(tokenizer.Number)},
	{typeRef(tokenizer.Dimension), typeRef(tokenizer.Percentage)},
	{typeRef(tokenizer.Dimension), typeRef(tokenizer.Dimension)},
	{typeRef(tokenizer.Dimension), typeRef(tokenizer.CDC)},

	{charRef('#'), typeRef(tokenizer.Ident)},
	{charRef('#'), typeRef(tokenizer.Function)},
	{charRef('#'), typeRef(tokenizer.Url)},
	{charRef('#'), typeRef(tokenizer.BadUrl)},
	{charRef('#'), charRef('-')},
	{charRef('#'), typeRef(tokenizer.Number)},
	{charRef('#'), typeRef(tokenizer.Percentage)},
	{charRef('#'), typeRef(tokenizer.Dimension)},
	{charRef('#'), typeRef(tokenizer.CDC)}, // https://github.com/w3c/csswg-drafts/pull/6874

	{charRef('-'), typeRef(tokenizer.Ident)},
	{charRef('-'), typeRef(tokenizer.Function)},
	{charRef('-'), typeRef(tokenizer.Url)},
	{charRef('-'), typeRef(tokenizer.BadUrl)},
	{charRef('-'), charRef('-')},
	{charRef('-'), typeRef(tokenizer.Number)},
	{charRef('-'), typeRef(tokenizer.Percentage)},
	{charRef('-'), typeRef(tokenizer.Dimension)},
	{charRef('-'), typeRef(tokenizer.CDC)}, // https://github.com/w3c/csswg-drafts/pull/6874

	{typeRef(tokenizer.Number), typeRef(tokenizer.Ident)},
	{typeRef(tokenizer.Number), typeRef(tokenizer.Function)},
	{typeRef(tokenizer.Number), typeRef(tokenizer.Url)},
	{typeRef(tokenizer.Number), typeRef(tokenizer.BadUrl)},
	{typeRef(tokenizer.Number), typeRef(tokenizer.Number)},
	{typeRef(tokenizer.Number), typeRef(tokenizer.Percentage)},
	{typeRef(tokenizer.Number), typeRef(tokenizer.Dimension)},
	{typeRef(tokenizer.Number), charRef('%')},
	{typeRef(tokenizer.Number), typeRef(tokenizer.CDC)}, // https://github.com/w3c/csswg-drafts/pull/6874

	{charRef('@'), typeRef(tokenizer.Ident)},
	{charRef('@'), typeRef(tokenizer.Function)},
	{charRef('@'), typeRef(tokenizer.Url)},
	{charRef('@'), typeRef(tokenizer.BadUrl)},
	{charRef('@'), charRef('-')},
	{charRef('@'), typeRef(tokenizer.CDC)}, // https://github.com/w3c/csswg-drafts/pull/6874

	{charRef('.'), typeRef(tokenizer.Number)},
	{charRef('.'), typeRef(tokenizer.Percentage)},
	{charRef('.'), typeRef(tokenizer.Dimension)},

	{charRef('+'), typeRef(tokenizer.Number)},
	{charRef('+'), typeRef(tokenizer.Percentage)},
	{charRef('+'), typeRef(tokenizer.Dimension)},

	{charRef('/'), charRef('*')},
}

// safePairs adds the pairs that are not required by the specification but keep browsers that read
// a serialization less strictly from running two tokens together.
var safePairs = append(append([]pair{}, specPairs...), []pair{
	{typeRef(tokenizer.Ident), typeRef(tokenizer.Hash)},

	{typeRef(tokenizer.Dimension), typeRef(tokenizer.Hash)},

	{typeRef(tokenizer.Hash), typeRef(tokenizer.Hash)},

	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.LeftParenthesis)},
	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.String)},
	{typeRef(tokenizer.AtKeyword), typeRef(tokenizer.Colon)},

	{typeRef(tokenizer.Percentage), typeRef(tokenizer.Percentage)},
	{typeRef(tokenizer.Percentage), typeRef(tokenizer.Dimension)},
	{typeRef(tokenizer.Percentage), typeRef(tokenizer.Function)},
	{typeRef(tokenizer.Percentage), charRef('-')},

	{typeRef(tokenizer.RightParenthesis), typeRef(tokenizer.Ident)},
	{typeRef(tokenizer.RightParenthesis), typeRef(tokenizer.Function)},
	{typeRef(tokenizer.RightParenthesis), typeRef(tokenizer.Percentage)},
	{typeRef(tokenizer.RightParenthesis), typeRef(tokenizer.Dimension)},
	{typeRef(tokenizer.RightParenthesis), typeRef(tokenizer.Hash)},
	{typeRef(tokenizer.RightParenthesis), charRef('-')},
}...)

var (
	specTokenBefore = createMap(specPairs)
	safeTokenBefore = createMap(safePairs)
)

// createMap turns a table of pairs into the function the generator asks before every token. It is
// given the code of the token written last and the token about to be written, and answers with the
// code of that token, its lowest bit set when a whitespace has to come first.
func createMap(pairs []pair) func(prevCode, tokenType int, value jsutil.Str) int {
	isWhiteSpaceRequired := make(map[int]bool, len(pairs))
	for _, p := range pairs {
		isWhiteSpaceRequired[p.prev.code()<<16|p.next.code()] = true
	}

	return func(prevCode, tokenType int, value jsutil.Str) int {
		nextCode := tokenCode(tokenType, value)
		nextCharCode := int(value.CharCodeAt(0))

		var emitWs bool
		if (nextCharCode == hyphenMinus &&
			tokenType != tokenizer.Ident &&
			tokenType != tokenizer.Function &&
			tokenType != tokenizer.CDC) ||
			nextCharCode == plusSign {
			emitWs = isWhiteSpaceRequired[(prevCode&0xFFFE)<<16|nextCharCode<<7]
		} else {
			emitWs = isWhiteSpaceRequired[(prevCode&0xFFFE)<<16|nextCode]
		}

		if emitWs {
			return nextCode | 1
		}
		return nextCode
	}
}
