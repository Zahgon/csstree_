package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

func (p *parser) consumeValueRaw() *ast.Node {
	return p.Raw(consumeUntilExclamationMarkOrSemicolon, true)
}

func (p *parser) consumeCustomPropertyRaw() *ast.Node {
	return p.Raw(consumeUntilExclamationMarkOrSemicolon, false)
}

// consumeValue reads a declaration value and makes sure nothing is left over before the end of
// the declaration.
func (p *parser) consumeValue() *ast.Node {
	startValueToken := p.TokenIndex
	value := p.Value()

	if value.Type != "Raw" &&
		!p.EOF &&
		p.TokenType != tokenizer.Semicolon &&
		!p.IsDelim(exclamationMark, 0) &&
		!p.IsBalanceEdge(startValueToken) {
		p.fail(nil)
	}

	return value
}

// isCustomPropertyName reports whether a property name begins with two hyphens.
func isCustomPropertyName(name jsutil.Str) bool {
	return len(name) >= 2 && name[0] == hyphenMinus && name[1] == hyphenMinus
}

// readProperty reads a property name, including the hacks a stylesheet may prefix it with.
func (p *parser) readProperty() jsutil.Str {
	start := p.TokenStart

	// hacks
	if p.TokenType == tokenizer.Delim {
		switch p.charCodeAt(p.TokenStart) {
		case asterisk, dollarSign, plusSign, numberSign, ampersand:
			p.Next()

		case solidus:
			p.Next()
			if p.IsDelim(solidus, 0) {
				p.Next()
			}
		}
	}

	if p.TokenType == tokenizer.Hash {
		p.eat(tokenizer.Hash)
	} else {
		p.eat(tokenizer.Ident)
	}

	return p.SubstrToCursor(start)
}

// getImportant reads the "! ws* important" ending, keeping the word as written when it is not
// "important", so that a hack such as "!ie" survives.
func (p *parser) getImportant() any {
	p.eat(tokenizer.Delim)
	p.SkipSC()

	important := p.consume(tokenizer.Ident)

	if important.EqualString("important") {
		return true
	}
	return important
}

// Declaration reads one declaration: a property, a value and an optional important ending.
func (p *parser) Declaration() *ast.Node {
	start := p.TokenStart
	startToken := p.TokenIndex
	property := p.readProperty()
	customProperty := isCustomPropertyName(property)

	parseValue := p.parseValue
	consumeRaw := p.consumeValueRaw
	if customProperty {
		parseValue = p.parseCustomProperty
		consumeRaw = p.consumeCustomPropertyRaw
	}

	var important any = false
	var value *ast.Node

	p.SkipSC()
	p.eat(tokenizer.Colon)

	valueStart := p.TokenIndex

	if !customProperty {
		p.SkipSC()
	}

	if parseValue {
		value = parseWithFallback(p, p.consumeValue, consumeRaw)
	} else {
		value = consumeRaw()
	}

	if customProperty && value.Type == "Value" && value.List().IsEmpty() {
		for offset := valueStart - p.TokenIndex; offset <= 0; offset++ {
			if p.LookupType(offset) == tokenizer.WhiteSpace {
				whiteSpace := ast.New("WhiteSpace")
				whiteSpace.SetString("value", " ")
				value.List().AppendData(whiteSpace)
				break
			}
		}
	}

	if p.IsDelim(exclamationMark, 0) {
		important = p.getImportant()
		p.SkipSC()
	}

	// Do not include semicolon to range per spec
	// https://drafts.csswg.org/css-syntax/#declaration-diagram
	if !p.EOF && p.TokenType != tokenizer.Semicolon && !p.IsBalanceEdge(startToken) {
		p.fail(nil)
	}

	node := ast.New("Declaration")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("important", important)
	node.Set("property", property)
	node.Set("value", value)
	return node
}

func (p *parser) consumeRawDeclarationList() *ast.Node {
	return p.Raw(consumeUntilSemicolonIncluded, true)
}

// DeclarationList reads the declarations, nested rules and at-rules of a block's content.
func (p *parser) DeclarationList() *ast.Node {
	children := ast.NewList()

	for !p.EOF {
		switch p.TokenType {
		case tokenizer.WhiteSpace, tokenizer.Comment, tokenizer.Semicolon:
			p.Next()

		case tokenizer.AtKeyword:
			children.AppendData(parseWithFallback(p, func() *ast.Node {
				return p.Atrule(true)
			}, p.consumeRawDeclarationList))

		default:
			if p.IsDelim(ampersand, 0) {
				children.AppendData(parseWithFallback(p, p.Rule, p.consumeRawDeclarationList))
			} else {
				children.AppendData(parseWithFallback(p, p.Declaration, p.consumeRawDeclarationList))
			}
		}
	}

	node := ast.New("DeclarationList")
	node.Loc = p.getLocationFromList(children)
	node.Set("children", children)
	return node
}

// Dimension reads a number with a unit.
func (p *parser) Dimension() *ast.Node {
	start := p.TokenStart
	value := p.consumeNumber(tokenizer.Dimension)

	node := ast.New("Dimension")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("value", value)
	node.Set("unit", p.substring(start+len(value), p.TokenStart))
	return node
}

// Function reads a function and its content, which a scope may have its own reader for.
func (p *parser) Function(readSequence sequenceFn, rec *recognizer) *ast.Node {
	start := p.TokenStart
	name := p.consumeFunctionName()
	nameLowerCase := name.ToLower().String()

	var children *ast.List
	if fn, ok := rec.fns[nameLowerCase]; ok {
		children = fn(p, rec)
	} else {
		children = readSequence(rec)
	}

	if !p.EOF {
		p.eat(tokenizer.RightParenthesis)
	}

	node := ast.New("Function")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("name", name)
	node.Set("children", children)
	return node
}
