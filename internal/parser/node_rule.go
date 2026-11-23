package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

// Raw reads a run of tokens as one unparsed node, up to the place the reader names or to the end
// of the enclosing block.
func (p *parser) Raw(consumeUntil func(code uint16) int, excludeWhiteSpace bool) *ast.Node {
	startOffset := p.GetTokenStart(p.TokenIndex)
	var endOffset int

	if consumeUntil == nil {
		consumeUntil = consumeUntilBalanceEnd
	}
	p.SkipUntilBalanced(p.TokenIndex, consumeUntil)

	if excludeWhiteSpace && p.TokenStart > startOffset {
		endOffset = p.offsetExcludeWS()
	} else {
		endOffset = p.TokenStart
	}

	node := ast.New("Raw")
	node.Loc = p.getLocation(startOffset, endOffset)
	node.Set("value", p.substring(startOffset, endOffset))
	return node
}

// offsetExcludeWS is where the run ends once the white space in front of the cursor is left out.
func (p *parser) offsetExcludeWS() int {
	if p.TokenIndex > 0 {
		if p.LookupType(-1) == tokenizer.WhiteSpace {
			if p.TokenIndex > 1 {
				return p.GetTokenStart(p.TokenIndex - 1)
			}
			return p.FirstCharOffset
		}
	}
	return p.TokenStart
}

func (p *parser) consumeRuleRaw() *ast.Node {
	return p.Raw(consumeUntilLeftCurlyBracket, true)
}

// consumeRulePrelude reads a rule's prelude and makes sure the block follows it.
func (p *parser) consumeRulePrelude() *ast.Node {
	prelude := p.SelectorList()

	if prelude.Type != "Raw" && !p.EOF && p.TokenType != tokenizer.LeftCurlyBracket {
		p.fail(nil)
	}

	return prelude
}

// Rule reads a qualified rule: a prelude and a block of declarations.
func (p *parser) Rule() *ast.Node {
	startOffset := p.TokenStart
	var prelude *ast.Node

	if p.parseRulePrelude {
		prelude = parseWithFallback(p, p.consumeRulePrelude, p.consumeRuleRaw)
	} else {
		prelude = p.consumeRuleRaw()
	}

	block := p.Block(true)

	node := ast.New("Rule")
	node.Loc = p.getLocation(startOffset, p.TokenStart)
	node.Set("prelude", prelude)
	node.Set("block", block)
	return node
}

func (p *parser) consumeBlockRaw() *ast.Node {
	return p.Raw(nil, true)
}

func (p *parser) consumeBlockRule() *ast.Node {
	return parseWithFallback(p, p.Rule, p.consumeBlockRaw)
}

func (p *parser) consumeBlockRawDeclaration() *ast.Node {
	return p.Raw(consumeUntilSemicolonIncluded, true)
}

// consumeBlockDeclaration reads one declaration of a style block and the semicolon after it.
func (p *parser) consumeBlockDeclaration() *ast.Node {
	if p.TokenType == tokenizer.Semicolon {
		return p.consumeBlockRawDeclaration()
	}

	node := parseWithFallback(p, p.Declaration, p.consumeBlockRawDeclaration)

	if p.TokenType == tokenizer.Semicolon {
		p.Next()
	}

	return node
}

// Block reads a curly brace block, either of declarations or of rules.
func (p *parser) Block(isStyleBlock bool) *ast.Node {
	consumer := p.consumeBlockRule
	if isStyleBlock {
		consumer = p.consumeBlockDeclaration
	}
	start := p.TokenStart
	children := ast.NewList()

	p.eat(tokenizer.LeftCurlyBracket)

scan:
	for !p.EOF {
		switch p.TokenType {
		case tokenizer.RightCurlyBracket:
			break scan

		case tokenizer.WhiteSpace, tokenizer.Comment:
			p.Next()

		case tokenizer.AtKeyword:
			children.AppendData(parseWithFallback(p, func() *ast.Node {
				return p.Atrule(isStyleBlock)
			}, p.consumeBlockRaw))

		default:
			if isStyleBlock && p.IsDelim(ampersand, 0) {
				children.AppendData(p.consumeBlockRule())
			} else {
				children.AppendData(consumer())
			}
		}
	}

	if !p.EOF {
		p.eat(tokenizer.RightCurlyBracket)
	}

	node := ast.New("Block")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("children", children)
	return node
}

// Brackets reads a square bracket group and its content.
func (p *parser) Brackets(readSequence sequenceFn, rec *recognizer) *ast.Node {
	start := p.TokenStart

	p.eat(tokenizer.LeftSquareBracket)

	children := readSequence(rec)

	if !p.EOF {
		p.eat(tokenizer.RightSquareBracket)
	}

	node := ast.New("Brackets")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("children", children)
	return node
}

// Parentheses reads a parenthesis group and its content.
func (p *parser) Parentheses(readSequence sequenceFn, rec *recognizer) *ast.Node {
	start := p.TokenStart

	p.eat(tokenizer.LeftParenthesis)

	children := readSequence(rec)

	if !p.EOF {
		p.eat(tokenizer.RightParenthesis)
	}

	node := ast.New("Parentheses")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("children", children)
	return node
}

// CDC reads the "-->" that ends a legacy HTML comment.
func (p *parser) CDC() *ast.Node {
	start := p.TokenStart

	p.eat(tokenizer.CDC)

	node := ast.New("CDC")
	node.Loc = p.getLocation(start, p.TokenStart)
	return node
}

// CDO reads the "<!--" that begins a legacy HTML comment.
func (p *parser) CDO() *ast.Node {
	start := p.TokenStart

	p.eat(tokenizer.CDO)

	node := ast.New("CDO")
	node.Loc = p.getLocation(start, p.TokenStart)
	return node
}

// Comment reads a comment, keeping the text between its delimiters.
func (p *parser) Comment() *ast.Node {
	start := p.TokenStart
	end := p.TokenEnd

	p.eat(tokenizer.Comment)

	if (end-start+2) >= 2 &&
		p.charCodeAt(end-2) == asterisk &&
		p.charCodeAt(end-1) == solidus {
		end -= 2
	}

	node := ast.New("Comment")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("value", p.substring(start+2, end))
	return node
}

// Percentage reads a number followed by a per cent sign.
func (p *parser) Percentage() *ast.Node {
	node := ast.New("Percentage")
	node.Loc = p.getLocation(p.TokenStart, p.TokenEnd)
	node.Set("value", p.consumeNumber(tokenizer.Percentage))
	return node
}

// consumeRatioTerm reads one side of a ratio.
func (p *parser) consumeRatioTerm() *ast.Node {
	p.SkipSC()

	switch p.TokenType {
	case tokenizer.Number:
		return p.Number()
	case tokenizer.Function:
		return p.Function(p.readSequence, p.scopeValue())
	}

	p.fail(jsutil.FromString("Number of function is expected"))
	return nil
}

// Ratio reads a ratio: a number, or two numbers with a solidus between them.
func (p *parser) Ratio() *ast.Node {
	start := p.TokenStart
	left := p.consumeRatioTerm()
	var right *ast.Node

	p.SkipSC()
	if p.IsDelim(solidus, 0) {
		p.eatDelim(solidus)
		right = p.consumeRatioTerm()
	}

	node := ast.New("Ratio")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("left", left)
	node.Set("right", nodeOrNull(right))
	return node
}

// Scope reads the prelude of an @scope rule: a root selector list and a limit.
func (p *parser) Scope() *ast.Node {
	var root *ast.Node
	var limit *ast.Node

	p.SkipSC()

	startOffset := p.TokenStart
	if p.TokenType == tokenizer.LeftParenthesis {
		p.Next()
		p.SkipSC()
		root = parseWithFallback(p, p.SelectorList, func() *ast.Node {
			return p.Raw(nil, true)
		})
		p.SkipSC()
		p.eat(tokenizer.RightParenthesis)
	}

	if p.lookupNonWSType(0) == tokenizer.Ident {
		p.SkipSC()
		p.eatIdent("to")
		p.SkipSC()
		p.eat(tokenizer.LeftParenthesis)
		p.SkipSC()
		limit = parseWithFallback(p, p.SelectorList, func() *ast.Node {
			return p.Raw(nil, true)
		})
		p.SkipSC()
		p.eat(tokenizer.RightParenthesis)
	}

	node := ast.New("Scope")
	node.Loc = p.getLocation(startOffset, p.TokenStart)
	node.Set("root", nodeOrNull(root))
	node.Set("limit", nodeOrNull(limit))
	return node
}

// SupportsDeclaration reads a parenthesised declaration of a supports condition.
func (p *parser) SupportsDeclaration() *ast.Node {
	start := p.TokenStart

	p.eat(tokenizer.LeftParenthesis)
	p.SkipSC()

	declaration := p.Declaration()

	if !p.EOF {
		p.eat(tokenizer.RightParenthesis)
	}

	node := ast.New("SupportsDeclaration")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("declaration", declaration)
	return node
}

func (p *parser) consumeStyleSheetRaw() *ast.Node {
	return p.Raw(nil, false)
}

// StyleSheet reads a whole style sheet.
func (p *parser) StyleSheet() *ast.Node {
	start := p.TokenStart
	children := ast.NewList()

	for !p.EOF {
		var child *ast.Node

		switch p.TokenType {
		case tokenizer.WhiteSpace:
			p.Next()
			continue

		case tokenizer.Comment:
			// ignore comments except exclamation comments (i.e. /*! .. */) on top level
			if p.charCodeAt(p.TokenStart+2) != exclamationMark {
				p.Next()
				continue
			}

			child = p.Comment()

		case tokenizer.CDO:
			child = p.CDO()

		case tokenizer.CDC:
			child = p.CDC()

		// CSS Syntax Module Level 3 §2.2 Error handling: at the top level of a style sheet an
		// at-keyword token starts an at-rule.
		case tokenizer.AtKeyword:
			child = parseWithFallback(p, func() *ast.Node {
				return p.Atrule(false)
			}, p.consumeStyleSheetRaw)

		// Anything else starts a qualified rule.
		default:
			child = parseWithFallback(p, p.Rule, p.consumeStyleSheetRaw)
		}

		children.AppendData(child)
	}

	node := ast.New("StyleSheet")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("children", children)
	return node
}

// Value reads a declaration value.
func (p *parser) Value() *ast.Node {
	start := p.TokenStart
	children := p.readSequence(p.scopeValue())

	node := ast.New("Value")
	node.Loc = p.getLocation(start, p.TokenStart)
	node.Set("children", children)
	return node
}

// whiteSpaceNode is the single node every run of white space inside a value becomes, as the
// original returns one frozen node for all of them.
func whiteSpaceNode() *ast.Node {
	node := ast.New("WhiteSpace")
	node.SetString("value", " ")
	return node
}

// WhiteSpace reads a run of white space inside a value.
func (p *parser) WhiteSpace() *ast.Node {
	p.eat(tokenizer.WhiteSpace)
	return whiteSpaceNode()
}
