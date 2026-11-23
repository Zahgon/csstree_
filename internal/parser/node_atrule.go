package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

func (p *parser) consumeAtruleRaw() *ast.Node {
	return p.Raw(consumeUntilLeftCurlyBracketOrSemicolon, true)
}

// isDeclarationBlockAtrule reports whether what follows reads as a block of declarations rather
// than of rules, which is what an at-rule with no reader of its own falls back on.
func (p *parser) isDeclarationBlockAtrule() bool {
	for offset := 1; ; offset++ {
		tokenType := p.LookupType(offset)
		if tokenType == tokenizer.EOF {
			return false
		}

		if tokenType == tokenizer.RightCurlyBracket {
			return true
		}

		if tokenType == tokenizer.LeftCurlyBracket || tokenType == tokenizer.AtKeyword {
			return false
		}
	}
}

// Atrule reads an at-rule: its name, its prelude and its block.
func (p *parser) Atrule(isDeclaration bool) *ast.Node {
	start := p.TokenStart
	var prelude *ast.Node
	var block *ast.Node

	p.eat(tokenizer.AtKeyword)

	name := p.SubstrToCursor(start + 1)
	nameLowerCase := name.ToLower().String()
	p.SkipSC()

	// parse prelude
	if !p.EOF && p.TokenType != tokenizer.LeftCurlyBracket && p.TokenType != tokenizer.Semicolon {
		if p.parseAtrulePrelude {
			prelude = parseWithFallback(p, func() *ast.Node {
				return p.AtrulePrelude(name)
			}, p.consumeAtruleRaw)
		} else {
			prelude = p.consumeAtruleRaw()
		}

		p.SkipSC()
	}

	switch p.TokenType {
	case tokenizer.Semicolon:
		p.Next()

	case tokenizer.LeftCurlyBracket:
		if def, ok := p.conf.atrule[nameLowerCase]; ok && def.block != nil {
			block = def.block(p, isDeclaration)
		} else {
			block = p.Block(p.isDeclarationBlockAtrule())
		}
	}

	node := ast.New("Atrule")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("name", name)
	node.Set("prelude", nodeOrNull(prelude))
	node.Set("block", nodeOrNull(block))
	return node
}

// AtrulePrelude reads what stands between an at-rule's name and its block or semicolon.
func (p *parser) AtrulePrelude(name jsutil.Str) *ast.Node {
	var children *ast.List

	lookup := ""
	if name != nil {
		lookup = name.ToLower().String()
	}

	p.SkipSC()

	if def, ok := p.conf.atrule[lookup]; ok && def.prelude != nil {
		// custom consumer
		children = def.prelude(p)
	} else {
		// default consumer
		children = p.readSequence(p.scopeAtrulePrelude())
	}

	p.SkipSC()

	if !p.EOF && p.TokenType != tokenizer.LeftCurlyBracket && p.TokenType != tokenizer.Semicolon {
		p.fail(jsutil.FromString("Semicolon or block is expected"))
	}

	node := ast.New("AtrulePrelude")
	node.Loc = p.getLocationFromList(children)
	node.Set("children", children)
	return node
}

// eatHexSequence reads the hexadecimal digits of a unicode range, answering how many it read, or
// -1 when it read a hyphen and the digits after it as well.
func (p *parser) eatHexSequence(offset int, allowDash bool) int {
	length := 0

	for pos := p.TokenStart + offset; pos < p.TokenEnd; pos++ {
		code := p.charCodeAt(pos)

		if code == hyphenMinus && allowDash && length != 0 {
			p.eatHexSequence(offset+length+1, false)
			return -1
		}

		if !tokenizer.IsHexDigit(code) {
			switch {
			case allowDash && length != 0 && length < 6:
				p.fail(jsutil.FromString("Hyphen minus or hex digit is expected"))
			case allowDash && length != 0:
				p.fail(jsutil.FromString("Hyphen minus is expected"))
			case length < 6:
				p.fail(jsutil.FromString("Hex digit is expected"))
			default:
				p.fail(jsutil.FromString("Unexpected input"))
			}
		}

		length++
		if length > 6 {
			p.failAt(jsutil.FromString("Too many hex digits"), pos)
		}
	}

	p.Next()
	return length
}

// eatQuestionMarkSequence reads the question marks that stand for any hex digit.
func (p *parser) eatQuestionMarkSequence(max int) {
	count := 0

	for p.IsDelim(questionMark, 0) {
		count++
		if count > max {
			p.fail(jsutil.FromString("Too many question marks"))
		}
		p.Next()
	}
}

// startsWith makes sure the current token begins with the code unit the microsyntax calls for.
func (p *parser) startsWith(code uint16) {
	if p.charCodeAt(p.TokenStart) != code {
		if code == plusSign {
			p.fail(jsutil.FromString("Plus sign is expected"))
		}
		p.fail(jsutil.FromString("Hyphen minus is expected"))
	}
}

// scanUnicodeRange reads the body of a unicode range, in each of its written forms.
// https://drafts.csswg.org/css-syntax/#urange
func (p *parser) scanUnicodeRange() {
	hexLength := 0

	switch p.TokenType {
	case tokenizer.Number:
		// u <number-token> '?'*
		// u <number-token> <dimension-token>
		// u <number-token> <number-token>
		hexLength = p.eatHexSequence(1, true)

		if p.IsDelim(questionMark, 0) {
			p.eatQuestionMarkSequence(6 - hexLength)
			break
		}

		if p.TokenType == tokenizer.Dimension || p.TokenType == tokenizer.Number {
			p.startsWith(hyphenMinus)
			p.eatHexSequence(1, false)
			break
		}

	case tokenizer.Dimension:
		// u <dimension-token> '?'*
		hexLength = p.eatHexSequence(1, true)
		if hexLength > 0 {
			p.eatQuestionMarkSequence(6 - hexLength)
		}

	default:
		// u '+' <ident-token> '?'*
		// u '+' '?'+
		p.eatDelim(plusSign)

		if p.TokenType == tokenizer.Ident {
			hexLength = p.eatHexSequence(0, true)
			if hexLength > 0 {
				p.eatQuestionMarkSequence(6 - hexLength)
			}
			break
		}

		if p.IsDelim(questionMark, 0) {
			p.Next()
			p.eatQuestionMarkSequence(5)
			break
		}

		p.fail(jsutil.FromString("Hex digit or question mark is expected"))
	}
}

// UnicodeRange reads a unicode range, written as u+<hex> with its several forms.
func (p *parser) UnicodeRange() *ast.Node {
	start := p.TokenStart

	// U or u
	p.eatIdent("u")
	p.scanUnicodeRange()

	node := ast.New("UnicodeRange")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("value", p.SubstrToCursor(start))
	return node
}
