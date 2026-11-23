package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

// scopeValue is the value scope, which a parser that has no value reader does not have; reaching
// for it then fails the way the original fails.
func (p *parser) scopeValue() *recognizer {
	if p.conf.scope.Value == nil {
		panic(typeError("Cannot read properties of undefined (reading 'Value')"))
	}
	return p.conf.scope.Value
}

func (p *parser) scopeSelector() *recognizer {
	if p.conf.scope.Selector == nil {
		panic(typeError("Cannot read properties of undefined (reading 'Selector')"))
	}
	return p.conf.scope.Selector
}

func (p *parser) scopeAtrulePrelude() *recognizer {
	if p.conf.scope.AtrulePrelude == nil {
		panic(typeError("Cannot read properties of undefined (reading 'AtrulePrelude')"))
	}
	return p.conf.scope.AtrulePrelude
}

// likelyFeatureToken is what may follow the name of a feature rather than of a range.
func isLikelyFeatureToken(tokenType int) bool {
	return tokenType == tokenizer.Colon || tokenType == tokenizer.RightParenthesis || tokenType == tokenizer.EOF
}

// featureOrRange tells a plain feature from a range by what follows the opening parenthesis.
func (p *parser) featureOrRange(kind jsutil.Str) *ast.Node {
	if p.lookupTypeNonSC(1) == tokenizer.Ident && isLikelyFeatureToken(p.lookupTypeNonSC(2)) {
		return p.Feature(kind)
	}
	return p.FeatureRange(kind)
}

// parenthesesReader is the reader a condition of this kind uses for a parenthesised term.
func (p *parser) parenthesesReader(kind jsutil.Str) func(jsutil.Str) *ast.Node {
	switch kind.String() {
	case "media", "container":
		return p.featureOrRange
	case "supports":
		return func(jsutil.Str) *ast.Node { return p.SupportsDeclaration() }
	}
	return nil
}

// lookupTypeNonSC is the type of the n-th token that is neither white space nor a comment.
func (p *parser) lookupTypeNonSC(idx int) int {
	return p.LookupTypeNonSC(idx)
}

// Condition reads the condition of a media query, a container query or a supports rule.
func (p *parser) Condition(kind jsutil.Str) *ast.Node {
	if kind == nil {
		kind = jsutil.FromString("media")
	}
	children := ast.NewList()

scan:
	for !p.EOF {
		switch p.TokenType {
		case tokenizer.Comment, tokenizer.WhiteSpace:
			p.Next()
			continue

		case tokenizer.Ident:
			children.AppendData(p.Identifier())

		case tokenizer.LeftParenthesis:
			term := parseWithFallback(p, func() *ast.Node {
				reader := p.parenthesesReader(kind)
				if reader == nil {
					panic(typeError("parentheses[kind] is not a function"))
				}
				return reader(kind)
			}, func() *ast.Node { return nil })

			if term == nil {
				term = parseWithFallback(p, func() *ast.Node {
					p.eat(tokenizer.LeftParenthesis)
					res := p.Condition(kind)
					p.eat(tokenizer.RightParenthesis)
					return res
				}, func() *ast.Node {
					return p.GeneralEnclosed(kind)
				})
			}

			children.AppendData(term)

		case tokenizer.Function:
			term := parseWithFallback(p, func() *ast.Node {
				return p.FeatureFunction(kind)
			}, func() *ast.Node { return nil })

			if term == nil {
				term = p.GeneralEnclosed(kind)
			}

			children.AppendData(term)

		default:
			break scan
		}
	}

	if children.IsEmpty() {
		p.fail(jsutil.FromString("Condition is expected"))
	}

	node := ast.New("Condition")
	node.Loc = p.getLocationFromList(children)
	node.Set("kind", kind)
	node.Set("children", children)
	return node
}

// Feature reads a parenthesised feature of a media or container query.
func (p *parser) Feature(kind jsutil.Str) *ast.Node {
	start := p.TokenStart
	var value *ast.Node

	p.eat(tokenizer.LeftParenthesis)
	p.SkipSC()

	name := p.consume(tokenizer.Ident)
	p.SkipSC()

	if p.TokenType != tokenizer.RightParenthesis {
		p.eat(tokenizer.Colon)
		p.SkipSC()

		switch p.TokenType {
		case tokenizer.Number:
			if p.lookupNonWSType(1) == tokenizer.Delim {
				value = p.Ratio()
			} else {
				value = p.Number()
			}

		case tokenizer.Dimension:
			value = p.Dimension()

		case tokenizer.Ident:
			value = p.Identifier()

		case tokenizer.Function:
			value = parseWithFallback(p, func() *ast.Node {
				res := p.Function(p.readSequence, p.scopeValue())

				p.SkipSC()

				if p.IsDelim(solidus, 0) {
					p.fail(nil)
				}

				return res
			}, func() *ast.Node {
				return p.Ratio()
			})

		default:
			p.fail(jsutil.FromString("Number, dimension, ratio or identifier is expected"))
		}

		p.SkipSC()
	}

	if !p.EOF {
		p.eat(tokenizer.RightParenthesis)
	}

	node := ast.New("Feature")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("kind", kind)
	node.Set("name", name)
	node.Set("value", nodeOrNull(value))
	return node
}

// FeatureFunction reads a function whose content is a selector or a declaration.
func (p *parser) FeatureFunction(kind jsutil.Str) *ast.Node {
	if kind == nil {
		kind = jsutil.FromString("unknown")
	}
	start := p.TokenStart
	functionName := p.consumeFunctionName()
	valueParser := p.featureParser(kind, functionName.ToLower())

	p.SkipSC()

	value := parseWithFallback(p, func() *ast.Node {
		startValueToken := p.TokenIndex
		result := valueParser(p)

		if !p.EOF && !p.IsBalanceEdge(startValueToken) {
			p.fail(nil)
		}

		return result
	}, func() *ast.Node {
		return p.Raw(nil, false)
	})

	if !p.EOF {
		p.eat(tokenizer.RightParenthesis)
	}

	node := ast.New("FeatureFunction")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("kind", kind)
	node.Set("feature", functionName)
	node.Set("value", value)
	return node
}

func (p *parser) featureParser(kind, name jsutil.Str) func(*parser) *ast.Node {
	featuresOfKind := p.conf.features[kind.String()]
	parse := featuresOfKind[name.String()]
	if parse == nil {
		p.fail(jsutil.Concat(jsutil.FromString("Unknown feature "), name, jsutil.FromString("()")))
	}
	return parse
}

// readTerm reads one side of a feature range.
func (p *parser) readRangeTerm() *ast.Node {
	p.SkipSC()

	switch p.TokenType {
	case tokenizer.Number:
		if p.IsDelim(solidus, p.LookupOffsetNonSC(1)) {
			return p.Ratio()
		}
		return p.Number()

	case tokenizer.Dimension:
		return p.Dimension()

	case tokenizer.Ident:
		return p.Identifier()

	case tokenizer.Function:
		return parseWithFallback(p, func() *ast.Node {
			res := p.Function(p.readSequence, p.scopeValue())
			p.SkipSC()
			if p.IsDelim(solidus, 0) {
				p.fail(nil)
			}
			return res
		}, func() *ast.Node {
			return p.Ratio()
		})
	}

	p.fail(jsutil.FromString("Number, dimension, ratio or identifier is expected"))
	return nil
}

// readComparison reads the comparison between two sides of a feature range.
func (p *parser) readComparison(expectColon bool) jsutil.Str {
	p.SkipSC()

	if p.IsDelim(lessThanSign, 0) || p.IsDelim(greaterThanSign, 0) {
		value := p.source.Substring(p.TokenStart, p.TokenStart+1)
		p.Next()

		if p.IsDelim(equalsSign, 0) {
			p.Next()
			return jsutil.Concat(value, jsutil.FromString("="))
		}

		return value
	}

	if p.IsDelim(equalsSign, 0) {
		return jsutil.FromString("=")
	}

	prefix := ""
	if expectColon {
		prefix = "\":\", "
	}
	p.fail(jsutil.FromString("Expected " + prefix + "\"<\", \">\", \"=\" or \")\""))
	return nil
}

// FeatureRange reads a parenthesised range of a media or container query.
func (p *parser) FeatureRange(kind jsutil.Str) *ast.Node {
	if kind == nil {
		kind = jsutil.FromString("unknown")
	}
	start := p.TokenStart

	p.SkipSC()
	p.eat(tokenizer.LeftParenthesis)

	left := p.readRangeTerm()
	leftComparison := p.readComparison(left.Type == "Identifier")
	middle := p.readRangeTerm()
	var rightComparison jsutil.Str
	var right *ast.Node

	if p.lookupNonWSType(0) != tokenizer.RightParenthesis {
		rightComparison = p.readComparison(false)
		right = p.readRangeTerm()
	}

	p.SkipSC()
	p.eat(tokenizer.RightParenthesis)

	node := ast.New("FeatureRange")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("kind", kind)
	node.Set("left", left)
	node.Set("leftComparison", leftComparison)
	node.Set("middle", middle)
	node.Set("rightComparison", strOrNull(rightComparison))
	node.Set("right", nodeOrNull(right))
	return node
}
