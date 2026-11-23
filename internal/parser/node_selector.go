package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/cssutil"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

// getAttributeName reads the name of an attribute selector, which may carry a namespace.
func (p *parser) getAttributeName() *ast.Node {
	if p.EOF {
		p.fail(jsutil.FromString("Unexpected end of input"))
	}

	start := p.TokenStart
	expectIdent := false

	if p.IsDelim(asterisk, 0) {
		expectIdent = true
		p.Next()
	} else if !p.IsDelim(verticalLine, 0) {
		p.eat(tokenizer.Ident)
	}

	if p.IsDelim(verticalLine, 0) {
		if p.charCodeAt(p.TokenStart+1) != equalsSign {
			p.Next()
			p.eat(tokenizer.Ident)
		} else if expectIdent {
			p.failAt(jsutil.FromString("Identifier is expected"), p.TokenEnd)
		}
	} else if expectIdent {
		p.fail(jsutil.FromString("Vertical line is expected"))
	}

	node := ast.New("Identifier")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("name", p.SubstrToCursor(start))
	return node
}

// getAttributeOperator reads the matcher of an attribute selector.
func (p *parser) getAttributeOperator() jsutil.Str {
	start := p.TokenStart
	code := p.charCodeAt(start)

	if code != equalsSign &&
		code != tilde &&
		code != circumflexAccent &&
		code != dollarSign &&
		code != asterisk &&
		code != verticalLine {
		p.fail(jsutil.FromString("Attribute selector (=, ~=, ^=, $=, *=, |=) is expected"))
	}

	p.Next()

	if code != equalsSign {
		if !p.IsDelim(equalsSign, 0) {
			p.fail(jsutil.FromString("Equal sign is expected"))
		}

		p.Next()
	}

	return p.SubstrToCursor(start)
}

// AttributeSelector reads an attribute selector.
func (p *parser) AttributeSelector() *ast.Node {
	start := p.TokenStart
	var matcher jsutil.Str
	var value *ast.Node
	var flags jsutil.Str

	p.eat(tokenizer.LeftSquareBracket)
	p.SkipSC()

	name := p.getAttributeName()
	p.SkipSC()

	if p.TokenType != tokenizer.RightSquareBracket {
		// avoid case `[name i]`
		if p.TokenType != tokenizer.Ident {
			matcher = p.getAttributeOperator()

			p.SkipSC()

			if p.TokenType == tokenizer.String {
				value = p.String()
			} else {
				value = p.Identifier()
			}

			p.SkipSC()
		}

		// attribute flags
		if p.TokenType == tokenizer.Ident {
			flags = p.consume(tokenizer.Ident)

			p.SkipSC()
		}
	}

	p.eat(tokenizer.RightSquareBracket)

	node := ast.New("AttributeSelector")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("name", name)
	node.Set("matcher", strOrNull(matcher))
	node.Set("value", nodeOrNull(value))
	node.Set("flags", strOrNull(flags))
	return node
}

// ClassSelector reads a class selector.
func (p *parser) ClassSelector() *ast.Node {
	p.eatDelim(fullStop)

	node := ast.New("ClassSelector")
	node.Loc = p.getLocation(p.TokenStart-1, p.TokenEnd)
	node.Set("name", p.consume(tokenizer.Ident))
	return node
}

// Combinator reads what joins two parts of a selector.
func (p *parser) Combinator() *ast.Node {
	start := p.TokenStart
	var name jsutil.Str

	switch p.TokenType {
	case tokenizer.WhiteSpace:
		name = jsutil.FromString(" ")

	case tokenizer.Delim:
		switch p.charCodeAt(p.TokenStart) {
		case greaterThanSign, plusSign, tilde:
			p.Next()

		case solidus:
			p.Next()
			p.eatIdent("deep")
			p.eatDelim(solidus)

		default:
			p.fail(jsutil.FromString("Combinator is expected"))
		}

		name = p.SubstrToCursor(start)
	}

	node := ast.New("Combinator")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("name", strOrNull(name))
	return node
}

// pseudoChildren reads the content of a pseudo class or element that takes an argument.
func (p *parser) pseudoChildren(nameLowerCase string) *ast.List {
	if p.lookupNonWSType(0) == tokenizer.RightParenthesis {
		return ast.NewList()
	}
	if read, ok := p.conf.pseudo[nameLowerCase]; ok {
		p.SkipSC()
		children := read(p)
		p.SkipSC()
		return children
	}
	return singleNodeList(p.Raw(nil, false))
}

// PseudoClassSelector reads a pseudo class, with its argument when it has one.
func (p *parser) PseudoClassSelector() *ast.Node {
	start := p.TokenStart
	var children *ast.List
	var name jsutil.Str

	p.eat(tokenizer.Colon)

	if p.TokenType == tokenizer.Function {
		name = p.consumeFunctionName()
		children = p.pseudoChildren(name.ToLower().String())
		p.eat(tokenizer.RightParenthesis)
	} else {
		name = p.consume(tokenizer.Ident)
	}

	node := ast.New("PseudoClassSelector")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("name", name)
	node.Set("children", listOrNull(children))
	return node
}

// PseudoElementSelector reads a pseudo element, with its argument when it has one.
func (p *parser) PseudoElementSelector() *ast.Node {
	start := p.TokenStart
	var children *ast.List
	var name jsutil.Str

	p.eat(tokenizer.Colon)
	p.eat(tokenizer.Colon)

	if p.TokenType == tokenizer.Function {
		name = p.consumeFunctionName()
		children = p.pseudoChildren(name.ToLower().String())
		p.eat(tokenizer.RightParenthesis)
	} else {
		name = p.consume(tokenizer.Ident)
	}

	node := ast.New("PseudoElementSelector")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("name", name)
	node.Set("children", listOrNull(children))
	return node
}

// Selector reads one selector of a selector list.
func (p *parser) Selector() *ast.Node {
	children := p.readSequence(p.scopeSelector())

	// nothing were consumed
	if children.First() == nil {
		p.fail(jsutil.FromString("Selector is expected"))
	}

	node := ast.New("Selector")
	node.Loc = p.getLocationFromList(children)
	node.Set("children", children)
	return node
}

// SelectorList reads a comma separated list of selectors.
func (p *parser) SelectorList() *ast.Node {
	children := ast.NewList()

	for !p.EOF {
		children.AppendData(p.Selector())

		if p.TokenType == tokenizer.Comma {
			p.Next()
			continue
		}

		break
	}

	node := ast.New("SelectorList")
	node.Loc = p.getLocationFromList(children)
	node.Set("children", children)
	return node
}

// eatIdentifierOrAsterisk reads one part of a type selector.
func (p *parser) eatIdentifierOrAsterisk() {
	if p.TokenType != tokenizer.Ident && !p.IsDelim(asterisk, 0) {
		p.fail(jsutil.FromString("Identifier or asterisk is expected"))
	}

	p.Next()
}

// TypeSelector reads an element selector, which may carry a namespace.
func (p *parser) TypeSelector() *ast.Node {
	start := p.TokenStart

	if p.IsDelim(verticalLine, 0) {
		p.Next()
		p.eatIdentifierOrAsterisk()
	} else {
		p.eatIdentifierOrAsterisk()

		if p.IsDelim(verticalLine, 0) {
			p.Next()
			p.eatIdentifierOrAsterisk()
		}
	}

	node := ast.New("TypeSelector")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("name", p.SubstrToCursor(start))
	return node
}

// String reads a quoted string and keeps the text it stands for.
func (p *parser) String() *ast.Node {
	node := ast.New("String")
	node.Loc = p.getLocation(p.TokenStart, p.TokenEnd)
	node.Set("value", cssutil.DecodeString(p.consume(tokenizer.String)))
	return node
}

// Url reads a url, written either as a url token or as url() around a string.
func (p *parser) Url() *ast.Node {
	start := p.TokenStart
	var value jsutil.Str

	switch p.TokenType {
	case tokenizer.Url:
		value = cssutil.DecodeURL(p.consume(tokenizer.Url))

	case tokenizer.Function:
		if !p.cmpStr(p.TokenStart, p.TokenEnd, "url(") {
			p.fail(jsutil.FromString("Function name must be `url`"))
		}

		p.eat(tokenizer.Function)
		p.SkipSC()
		value = cssutil.DecodeString(p.consume(tokenizer.String))
		p.SkipSC()
		if !p.EOF {
			p.eat(tokenizer.RightParenthesis)
		}

	default:
		p.fail(jsutil.FromString("Url or Function is expected"))
	}

	node := ast.New("Url")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("value", value)
	return node
}
