package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

// singleNodeList is a list holding one node, which several readers build for a fallback.
func singleNodeList(node *ast.Node) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{node})
}

// GeneralEnclosed reads a parenthesised or function-wrapped run of any value at all, which is
// what a condition falls back to when it understands neither the feature nor the function.
func (p *parser) GeneralEnclosed(kind jsutil.Str) *ast.Node {
	start := p.TokenStart
	var functionName jsutil.Str

	if p.TokenType == tokenizer.Function {
		functionName = p.consumeFunctionName()
	} else {
		p.eat(tokenizer.LeftParenthesis)
	}

	children := parseWithFallback(p, func() *ast.List {
		startValueToken := p.TokenIndex
		list := p.readSequence(p.scopeValue())

		if !p.EOF && !p.IsBalanceEdge(startValueToken) {
			p.fail(nil)
		}

		return list
	}, func() *ast.List {
		return singleNodeList(p.Raw(nil, false))
	})

	if !p.EOF {
		p.eat(tokenizer.RightParenthesis)
	}

	node := ast.New("GeneralEnclosed")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("kind", kind)
	node.Set("function", strOrNull(functionName))
	node.Set("children", children)
	return node
}

// Hash reads a hash token as a value, such as a colour.
func (p *parser) Hash() *ast.Node {
	start := p.TokenStart

	p.eat(tokenizer.Hash)

	node := ast.New("Hash")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("value", p.SubstrToCursor(start+1))
	return node
}

// IdSelector reads an id selector.
func (p *parser) IdSelector() *ast.Node {
	start := p.TokenStart

	p.eat(tokenizer.Hash)

	node := ast.New("IdSelector")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("name", p.SubstrToCursor(start+1))
	return node
}

// Identifier reads an identifier.
func (p *parser) Identifier() *ast.Node {
	node := ast.New("Identifier")
	node.Loc = p.getLocation(p.TokenStart, p.TokenEnd)
	node.Set("name", p.consume(tokenizer.Ident))
	return node
}

// Layer reads a cascade layer name, which may be several identifiers joined by full stops.
func (p *parser) Layer() *ast.Node {
	tokenStart := p.TokenStart
	name := p.consume(tokenizer.Ident)

	for p.IsDelim(fullStop, 0) {
		p.eat(tokenizer.Delim)
		name = jsutil.Concat(name, jsutil.FromString("."), p.consume(tokenizer.Ident))
	}

	node := ast.New("Layer")
	node.Loc = p.getLocation(tokenStart, p.TokenStart)
	node.Set("name", name)
	return node
}

// LayerList reads a comma separated list of cascade layer names.
func (p *parser) LayerList() *ast.Node {
	children := ast.NewList()

	p.SkipSC()

	for !p.EOF {
		children.AppendData(p.Layer())

		if p.LookupTypeNonSC(0) != tokenizer.Comma {
			break
		}

		p.SkipSC()
		p.Next()
		p.SkipSC()
	}

	node := ast.New("LayerList")
	node.Loc = p.getLocationFromList(children)
	node.Set("children", children)
	return node
}

// MediaQuery reads one media query: an optional modifier, a media type and a condition.
func (p *parser) MediaQuery() *ast.Node {
	start := p.TokenStart
	var modifier jsutil.Str
	var mediaType jsutil.Str
	var condition *ast.Node

	p.SkipSC()

	if p.TokenType == tokenizer.Ident && p.LookupTypeNonSC(1) != tokenizer.LeftParenthesis {
		// [ not | only ]? <media-type>
		ident := p.consume(tokenizer.Ident)
		identLowerCase := ident.ToLower()

		if identLowerCase.EqualString("not") || identLowerCase.EqualString("only") {
			p.SkipSC()
			modifier = identLowerCase
			mediaType = p.consume(tokenizer.Ident)
		} else {
			mediaType = ident
		}

		switch p.LookupTypeNonSC(0) {
		case tokenizer.Ident:
			// and <media-condition-without-or>
			p.SkipSC()
			p.eatIdent("and")
			condition = p.Condition(jsutil.FromString("media"))

		case tokenizer.LeftCurlyBracket, tokenizer.Semicolon, tokenizer.Comma, tokenizer.EOF:

		default:
			p.fail(jsutil.FromString("Identifier or parenthesis is expected"))
		}
	} else {
		switch p.TokenType {
		case tokenizer.Ident, tokenizer.LeftParenthesis, tokenizer.Function:
			// <media-condition>
			condition = p.Condition(jsutil.FromString("media"))

		case tokenizer.LeftCurlyBracket, tokenizer.Semicolon, tokenizer.EOF:

		default:
			p.fail(jsutil.FromString("Identifier or parenthesis is expected"))
		}
	}

	node := ast.New("MediaQuery")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("modifier", strOrNull(modifier))
	node.Set("mediaType", strOrNull(mediaType))
	node.Set("condition", nodeOrNull(condition))
	return node
}

// MediaQueryList reads a comma separated list of media queries.
func (p *parser) MediaQueryList() *ast.Node {
	children := ast.NewList()

	p.SkipSC()

	for !p.EOF {
		children.AppendData(p.MediaQuery())

		if p.TokenType != tokenizer.Comma {
			break
		}

		p.Next()
	}

	node := ast.New("MediaQueryList")
	node.Loc = p.getLocationFromList(children)
	node.Set("children", children)
	return node
}

// NestingSelector reads the ampersand that stands for the enclosing rule's selector.
func (p *parser) NestingSelector() *ast.Node {
	start := p.TokenStart

	p.eatDelim(ampersand)

	node := ast.New("NestingSelector")
	node.Loc = p.getLocation(start, p.TokenStart)
	return node
}

// Nth reads the argument of :nth-child() and its kin: an An+B value or a keyword, and an
// optional "of" selector list.
func (p *parser) Nth() *ast.Node {
	p.SkipSC()

	start := p.TokenStart
	end := start
	var selector *ast.Node
	var nth *ast.Node

	if p.LookupValue(0, "odd") || p.LookupValue(0, "even") {
		nth = p.Identifier()
	} else {
		nth = p.AnPlusB()
	}

	end = p.TokenStart
	p.SkipSC()

	if p.LookupValue(0, "of") {
		p.Next()

		selector = p.SelectorList()
		end = p.TokenStart
	}

	node := ast.New("Nth")
	node.Loc = p.getLocation(start, end)
	node.Set("nth", nth)
	node.Set("selector", nodeOrNull(selector))
	return node
}

// Number reads a number.
func (p *parser) Number() *ast.Node {
	node := ast.New("Number")
	node.Loc = p.getLocation(p.TokenStart, p.TokenEnd)
	node.Set("value", p.consume(tokenizer.Number))
	return node
}

// Operator reads a single token that separates two parts of a value.
func (p *parser) Operator() *ast.Node {
	start := p.TokenStart

	p.Next()

	node := ast.New("Operator")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("value", p.SubstrToCursor(start))
	return node
}
