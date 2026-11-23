package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

const (
	disallowSign = true
	allowSign    = false
)

// checkInteger makes sure the rest of the current token, from the offset on, reads as an integer.
func (p *parser) checkInteger(offset int, disallow bool) {
	pos := p.TokenStart + offset
	code := p.charCodeAt(pos)

	if code == plusSign || code == hyphenMinus {
		if disallow {
			p.fail(jsutil.FromString("Number sign is not allowed"))
		}
		pos++
	}

	for ; pos < p.TokenEnd; pos++ {
		if !tokenizer.IsDigit(p.charCodeAt(pos)) {
			p.failAt(jsutil.FromString("Integer is expected"), pos)
		}
	}
}

func (p *parser) checkTokenIsInteger(disallow bool) {
	p.checkInteger(0, disallow)
}

// expectCharCode makes sure the code unit at the offset inside the current token is the one the
// An+B microsyntax calls for there.
func (p *parser) expectCharCode(offset int, code uint16) {
	if !p.cmpChar(p.TokenStart+offset, code) {
		message := ""

		switch code {
		case letterN:
			message = "N is expected"
		case hyphenMinus:
			message = "HyphenMinus is expected"
		}

		p.failAt(jsutil.FromString(message), p.TokenStart+offset)
	}
}

// consumeB reads the "+ b" part of an An+B value: a signed integer, or a sign and a signless one.
func (p *parser) consumeB() jsutil.Str {
	offset := 0
	sign := uint16(0)
	tokenType := p.TokenType

	for tokenType == tokenizer.WhiteSpace || tokenType == tokenizer.Comment {
		offset++
		tokenType = p.LookupType(offset)
	}

	if tokenType != tokenizer.Number {
		if p.IsDelim(plusSign, offset) || p.IsDelim(hyphenMinus, offset) {
			if p.IsDelim(plusSign, offset) {
				sign = plusSign
			} else {
				sign = hyphenMinus
			}

			for {
				offset++
				tokenType = p.LookupType(offset)
				if tokenType != tokenizer.WhiteSpace && tokenType != tokenizer.Comment {
					break
				}
			}

			if tokenType != tokenizer.Number {
				p.Skip(offset)
				p.checkTokenIsInteger(disallowSign)
			}
		} else {
			return nil
		}
	}

	if offset > 0 {
		p.Skip(offset)
	}

	if sign == 0 {
		code := p.charCodeAt(p.TokenStart)
		if code != plusSign && code != hyphenMinus {
			p.fail(jsutil.FromString("Number sign is expected"))
		}
	}

	p.checkTokenIsInteger(sign != 0)

	if sign == hyphenMinus {
		return jsutil.Concat(jsutil.FromString("-"), p.consume(tokenizer.Number))
	}
	return p.consume(tokenizer.Number)
}

// AnPlusB reads the An+B microsyntax: https://www.w3.org/TR/css-syntax-3/#anb
func (p *parser) AnPlusB() *ast.Node {
	start := p.TokenStart
	var a jsutil.Str
	var b jsutil.Str

	switch {
	// <integer>
	case p.TokenType == tokenizer.Number:
		p.checkTokenIsInteger(allowSign)
		b = p.consume(tokenizer.Number)

	// -n
	// -n <signed-integer>
	// -n ['+' | '-'] <signless-integer>
	// -n- <signless-integer>
	// <dashndashdigit-ident>
	case p.TokenType == tokenizer.Ident && p.cmpChar(p.TokenStart, hyphenMinus):
		a = jsutil.FromString("-1")

		p.expectCharCode(1, letterN)

		switch p.TokenEnd - p.TokenStart {
		// -n
		// -n <signed-integer>
		// -n ['+' | '-'] <signless-integer>
		case 2:
			p.Next()
			b = p.consumeB()

		// -n- <signless-integer>
		case 3:
			p.expectCharCode(2, hyphenMinus)

			p.Next()
			p.SkipSC()

			p.checkTokenIsInteger(disallowSign)

			b = jsutil.Concat(jsutil.FromString("-"), p.consume(tokenizer.Number))

		// <dashndashdigit-ident>
		default:
			p.expectCharCode(2, hyphenMinus)
			p.checkInteger(3, disallowSign)
			p.Next()

			b = p.SubstrToCursor(start + 2)
		}

	// '+'? n
	// '+'? n <signed-integer>
	// '+'? n ['+' | '-'] <signless-integer>
	// '+'? n- <signless-integer>
	// '+'? <ndashdigit-ident>
	case p.TokenType == tokenizer.Ident || (p.IsDelim(plusSign, 0) && p.LookupType(1) == tokenizer.Ident):
		sign := 0
		a = jsutil.FromString("1")

		// just ignore a plus
		if p.IsDelim(plusSign, 0) {
			sign = 1
			p.Next()
		}

		p.expectCharCode(0, letterN)

		switch p.TokenEnd - p.TokenStart {
		// '+'? n
		// '+'? n <signed-integer>
		// '+'? n ['+' | '-'] <signless-integer>
		case 1:
			p.Next()
			b = p.consumeB()

		// '+'? n- <signless-integer>
		case 2:
			p.expectCharCode(1, hyphenMinus)

			p.Next()
			p.SkipSC()

			p.checkTokenIsInteger(disallowSign)

			b = jsutil.Concat(jsutil.FromString("-"), p.consume(tokenizer.Number))

		// '+'? <ndashdigit-ident>
		default:
			p.expectCharCode(1, hyphenMinus)
			p.checkInteger(2, disallowSign)
			p.Next()

			b = p.SubstrToCursor(start + sign + 1)
		}

	// <ndashdigit-dimension>
	// <ndash-dimension> <signless-integer>
	// <n-dimension>
	// <n-dimension> <signed-integer>
	// <n-dimension> ['+' | '-'] <signless-integer>
	case p.TokenType == tokenizer.Dimension:
		code := p.charCodeAt(p.TokenStart)
		sign := 0
		if code == plusSign || code == hyphenMinus {
			sign = 1
		}
		i := p.TokenStart + sign

		for ; i < p.TokenEnd; i++ {
			if !tokenizer.IsDigit(p.charCodeAt(i)) {
				break
			}
		}

		if i == p.TokenStart+sign {
			p.failAt(jsutil.FromString("Integer is expected"), p.TokenStart+sign)
		}

		p.expectCharCode(i-p.TokenStart, letterN)
		a = p.substring(start, i)

		// <n-dimension>
		// <n-dimension> <signed-integer>
		// <n-dimension> ['+' | '-'] <signless-integer>
		if i+1 == p.TokenEnd {
			p.Next()
			b = p.consumeB()
		} else {
			p.expectCharCode(i-p.TokenStart+1, hyphenMinus)

			if i+2 == p.TokenEnd {
				// <ndash-dimension> <signless-integer>
				p.Next()
				p.SkipSC()
				p.checkTokenIsInteger(disallowSign)
				b = jsutil.Concat(jsutil.FromString("-"), p.consume(tokenizer.Number))
			} else {
				// <ndashdigit-dimension>
				p.checkInteger(i-p.TokenStart+2, disallowSign)
				p.Next()
				b = p.SubstrToCursor(i + 1)
			}
		}

	default:
		p.fail(nil)
	}

	if a != nil && a.CharCodeAt(0) == plusSign {
		a = a.Slice(1)
	}

	if b != nil && b.CharCodeAt(0) == plusSign {
		b = b.Slice(1)
	}

	node := ast.New("AnPlusB")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("a", strOrNull(a))
	node.Set("b", strOrNull(b))
	return node
}
