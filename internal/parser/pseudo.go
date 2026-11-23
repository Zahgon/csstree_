package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

func pseudoSelectorList(p *parser) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{p.SelectorList()})
}

func pseudoSelector(p *parser) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{p.Selector()})
}

func pseudoIdentList(p *parser) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{p.Identifier()})
}

func pseudoNth(p *parser) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{p.Nth()})
}

// parseLanguageRangeList reads the content of lang(), which is identifiers and strings separated
// by commas.
func parseLanguageRangeList(p *parser) *ast.List {
	children := ast.NewList()

	p.SkipSC()

loop:
	for !p.EOF {
		switch p.TokenType {
		case tokenizer.Ident:
			children.AppendData(p.Identifier())

		case tokenizer.String:
			children.AppendData(p.String())

		case tokenizer.Comma:
			children.AppendData(p.Operator())

		case tokenizer.RightParenthesis:
			break loop

		default:
			p.fail(jsutil.FromString("Identifier, string or comma is expected"))
		}

		p.SkipSC()
	}

	return children
}

// pseudos are the pseudo classes and pseudo elements whose content is read as a tree rather than
// left raw.
var pseudos = map[string]func(p *parser) *ast.List{
	"dir":              pseudoIdentList,
	"has":              pseudoSelectorList,
	"lang":             parseLanguageRangeList,
	"matches":          pseudoSelectorList,
	"is":               pseudoSelectorList,
	"-moz-any":         pseudoSelectorList,
	"-webkit-any":      pseudoSelectorList,
	"where":            pseudoSelectorList,
	"not":              pseudoSelectorList,
	"nth-child":        pseudoNth,
	"nth-last-child":   pseudoNth,
	"nth-last-of-type": pseudoNth,
	"nth-of-type":      pseudoNth,
	"slotted":          pseudoSelector,
	"host":             pseudoSelector,
	"host-context":     pseudoSelector,
}
