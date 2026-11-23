package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

// atruleDef is what an at-rule name adds to the parser: a reader for its prelude and a reader for
// its block. Either may be missing, which means the at-rule has none of that part or reads it the
// default way.
type atruleDef struct {
	prelude func(p *parser) *ast.List
	block   func(p *parser, nested bool) *ast.Node
}

func blockNested(p *parser, nested bool) *ast.Node {
	return p.Block(nested)
}

func blockStyle(p *parser, nested bool) *ast.Node {
	return p.Block(true)
}

func blockRules(p *parser, nested bool) *ast.Node {
	return p.Block(false)
}

// nonContainerNameKeywords are the words a container name may not be.
// https://drafts.csswg.org/css-contain-3/#container-rule
var nonContainerNameKeywords = map[string]bool{"none": true, "and": true, "not": true, "or": true}

func containerPrelude(p *parser) *ast.List {
	children := ast.NewList()

	if p.TokenType == tokenizer.Ident {
		name := p.substring(p.TokenStart, p.TokenEnd)

		if !nonContainerNameKeywords[name.ToLower().String()] {
			children.AppendData(p.Identifier())
		}
	}

	children.AppendData(p.Condition(jsutil.FromString("container")))

	return children
}

func layerPrelude(p *parser) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{p.LayerList()})
}

func mediaPrelude(p *parser) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{p.MediaQueryList()})
}

func selectorListPrelude(p *parser) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{p.SelectorList()})
}

func scopePrelude(p *parser) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{p.Scope()})
}

func supportsPrelude(p *parser) *ast.List {
	return ast.NewList().FromSlice([]*ast.Node{p.Condition(jsutil.FromString("supports"))})
}

// importParseWithFallback reads one part of an @import prelude and makes sure the whole of it was
// read, since anything left over before the closing parenthesis means the part was not understood.
func importParseWithFallback(p *parser, parse func() *ast.Node, fallback func() *ast.Node) *ast.Node {
	if fallback == nil {
		fallback = func() *ast.Node {
			return p.Raw(nil, true)
		}
	}

	return parseWithFallback(p, func() (result *ast.Node) {
		defer func() {
			p.SkipSC()
			if p.lookupNonWSType(0) != tokenizer.RightParenthesis {
				p.fail(nil)
			}
		}()
		return parse()
	}, fallback)
}

// importLayerFn reads the content of layer() in an @import prelude.
func importLayerFn(p *parser, rec *recognizer) *ast.List {
	p.SkipSC()

	children := ast.NewList()
	node := importParseWithFallback(p, p.Layer, nil)

	if node.Type != "Raw" || len(node.Str("value")) != 0 {
		children.AppendData(node)
	}

	return children
}

// importSupportsFn reads the content of supports() in an @import prelude.
func importSupportsFn(p *parser, rec *recognizer) *ast.List {
	p.SkipSC()

	children := ast.NewList()
	node := importParseWithFallback(p, p.Declaration, func() *ast.Node {
		return importParseWithFallback(p, func() *ast.Node {
			return p.Condition(jsutil.FromString("supports"))
		}, nil)
	})

	if node.Type != "Raw" || len(node.Str("value")) != 0 {
		children.AppendData(node)
	}

	return children
}

// importFunctions are the functions an @import prelude knows how to read.
var importFunctions = &recognizer{
	fns: map[string]func(p *parser, rec *recognizer) *ast.List{
		"layer":    importLayerFn,
		"supports": importSupportsFn,
	},
}

func importPrelude(p *parser) *ast.List {
	children := ast.NewList()

	switch p.TokenType {
	case tokenizer.String:
		children.AppendData(p.String())

	case tokenizer.Url, tokenizer.Function:
		children.AppendData(p.Url())

	default:
		p.fail(jsutil.FromString("String or url() is expected"))
	}

	p.SkipSC()

	if p.TokenType == tokenizer.Ident && p.cmpStr(p.TokenStart, p.TokenEnd, "layer") {
		children.AppendData(p.Identifier())
	} else if p.TokenType == tokenizer.Function && p.cmpStr(p.TokenStart, p.TokenEnd, "layer(") {
		children.AppendData(p.Function(nil, importFunctions))
	}

	p.SkipSC()

	if p.TokenType == tokenizer.Function && p.cmpStr(p.TokenStart, p.TokenEnd, "supports(") {
		children.AppendData(p.Function(nil, importFunctions))
	}

	if p.lookupNonWSType(0) == tokenizer.Ident || p.lookupNonWSType(0) == tokenizer.LeftParenthesis {
		children.AppendData(p.MediaQueryList())
	}

	return children
}

// atrules are the at-rules that read their prelude or their block their own way.
var atrules = map[string]atruleDef{
	"container":      {prelude: containerPrelude, block: blockNested},
	"font-face":      {prelude: nil, block: blockStyle},
	"import":         {prelude: importPrelude, block: nil},
	"layer":          {prelude: layerPrelude, block: blockRules},
	"media":          {prelude: mediaPrelude, block: blockNested},
	"nest":           {prelude: selectorListPrelude, block: blockStyle},
	"page":           {prelude: selectorListPrelude, block: blockStyle},
	"scope":          {prelude: scopePrelude, block: blockNested},
	"starting-style": {prelude: nil, block: blockNested},
	"supports":       {prelude: supportsPrelude, block: blockNested},
}
