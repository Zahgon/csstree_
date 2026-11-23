package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

// defaultGetNode reads the next node of a value or an at-rule prelude.
func defaultGetNode(p *parser, ctx *seqContext) *ast.Node {
	switch p.TokenType {
	case tokenizer.Hash:
		return p.Hash()

	case tokenizer.Comma:
		return p.Operator()

	case tokenizer.LeftParenthesis:
		return p.Parentheses(p.readSequence, ctx.recognizer)

	case tokenizer.LeftSquareBracket:
		return p.Brackets(p.readSequence, ctx.recognizer)

	case tokenizer.String:
		return p.String()

	case tokenizer.Dimension:
		return p.Dimension()

	case tokenizer.Percentage:
		return p.Percentage()

	case tokenizer.Number:
		return p.Number()

	case tokenizer.Function:
		if p.cmpStr(p.TokenStart, p.TokenEnd, "url(") {
			return p.Url()
		}
		return p.Function(p.readSequence, ctx.recognizer)

	case tokenizer.Url:
		return p.Url()

	case tokenizer.Ident:
		// check for unicode range, it should start with u+ or U+
		if p.cmpChar(p.TokenStart, letterU) && p.cmpChar(p.TokenStart+1, plusSign) {
			return p.UnicodeRange()
		}
		return p.Identifier()

	case tokenizer.Delim:
		code := p.charCodeAt(p.TokenStart)

		if code == solidus || code == asterisk || code == plusSign || code == hyphenMinus {
			return p.Operator()
		}

		if code == numberSign {
			p.failAt(jsutil.FromString("Hex or identifier is expected"), p.TokenStart+1)
		}
	}

	return nil
}

// selectorGetNode reads the next part of a selector.
func selectorGetNode(p *parser, ctx *seqContext) *ast.Node {
	switch p.TokenType {
	case tokenizer.LeftSquareBracket:
		return p.AttributeSelector()

	case tokenizer.Hash:
		return p.IdSelector()

	case tokenizer.Colon:
		if p.LookupType(1) == tokenizer.Colon {
			return p.PseudoElementSelector()
		}
		return p.PseudoClassSelector()

	case tokenizer.Ident:
		return p.TypeSelector()

	case tokenizer.Number, tokenizer.Percentage:
		return p.Percentage()

	case tokenizer.Dimension:
		// throws when .123ident
		if p.charCodeAt(p.TokenStart) == fullStop {
			p.failAt(jsutil.FromString("Identifier is expected"), p.TokenStart+1)
		}

	case tokenizer.Delim:
		switch p.charCodeAt(p.TokenStart) {
		case plusSign, greaterThanSign, tilde, solidus: // /deep/
			return p.Combinator()

		case fullStop:
			return p.ClassSelector()

		case asterisk, verticalLine:
			return p.TypeSelector()

		case numberSign:
			return p.IdSelector()

		case ampersand:
			return p.NestingSelector()
		}
	}

	return nil
}

// selectorOnWhiteSpace puts a descendant combinator where white space separates two parts of a
// selector that are not combinators themselves.
func selectorOnWhiteSpace(p *parser, next *ast.Node, children *ast.List, ctx *seqContext) {
	last := children.Last()

	if last != nil && last.Type != "Combinator" && next != nil && next.Type != "Combinator" {
		// The descendant combinator is built here rather than read from the stream, because the
		// white space it stands for has already been passed over.
		children.AppendData(ast.New("Combinator").SetString("name", " "))
	}
}

// isPlusMinusOperator reports whether a node is an operator whose text ends in a sign, which is
// the one case where white space inside a value has to be kept.
func isPlusMinusOperator(node *ast.Node) bool {
	if node == nil || node.Type != "Operator" {
		return false
	}
	value := node.Str("value")
	if len(value) == 0 {
		return false
	}
	last := value[len(value)-1]
	return last == hyphenMinus || last == plusSign
}

func valueOnWhiteSpace(p *parser, next *ast.Node, children *ast.List, ctx *seqContext) {
	if isPlusMinusOperator(next) {
		next.Set("value", jsutil.Concat(jsutil.Str{' '}, next.Str("value")))
	}
	if last := children.Last(); isPlusMinusOperator(last) {
		last.Set("value", jsutil.Concat(last.Str("value"), jsutil.Str{' '}))
	}
}

// expressionFn reads the content of the legacy expression() function, which is any value at all.
func expressionFn(p *parser, rec *recognizer) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{p.Raw(nil, false)})
}

// varFn reads the content of var(), which is a custom property name and an optional fallback.
func varFn(p *parser, rec *recognizer) *ast.List {
	children := ast.NewList()

	p.SkipSC()

	// NOTE: Don't check more than a first argument is an ident, rest checks are for lexer
	children.AppendData(p.Identifier())

	p.SkipSC()

	if p.TokenType == tokenizer.Comma {
		children.AppendData(p.Operator())

		startIndex := p.TokenIndex
		var value *ast.Node
		if p.parseCustomProperty {
			value = p.Value()
		} else {
			value = p.Raw(consumeUntilExclamationMarkOrSemicolon, false)
		}

		if value.Type == "Value" && value.List().IsEmpty() {
			for offset := startIndex - p.TokenIndex; offset <= 0; offset++ {
				if p.LookupType(offset) == tokenizer.WhiteSpace {
					value.List().AppendData(ast.New("WhiteSpace").SetString("value", " "))
					break
				}
			}
		}

		children.AppendData(value)
	}

	return children
}

var (
	// atrulePreludeScope reads an at-rule prelude that has no reader of its own.
	atrulePreludeScope = &recognizer{
		getNode: defaultGetNode,
	}

	// selectorScope reads a selector.
	selectorScope = &recognizer{
		getNode:      selectorGetNode,
		onWhiteSpace: selectorOnWhiteSpace,
	}

	// valueScope reads a declaration value.
	valueScope = &recognizer{
		getNode:      defaultGetNode,
		onWhiteSpace: valueOnWhiteSpace,
		fns: map[string]func(p *parser, rec *recognizer) *ast.List{
			"expression": expressionFn,
			"var":        varFn,
		},
	}
)
