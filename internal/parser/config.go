package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
)

// contextFn reads the piece of CSS one parse context names.
type contextFn func(p *parser, opts Options) *ast.Node

// scopeSet holds the three scopes a sequence can be read in. A parser that has no use for one of
// them leaves it out, and reaching for it then fails the way the original fails.
type scopeSet struct {
	AtrulePrelude *recognizer
	Selector      *recognizer
	Value         *recognizer
}

// config is the assembled parser configuration: the parse contexts, the feature functions, the
// scopes, the at-rules and the pseudos.
type config struct {
	contexts map[string]contextFn
	features map[string]map[string]func(p *parser) *ast.Node
	scope    scopeSet
	atrule   map[string]atruleDef
	pseudo   map[string]func(p *parser) *ast.List
}

// features are the functions a condition can hold whose content is a selector or a declaration.
var features = map[string]map[string]func(p *parser) *ast.Node{
	"supports": {
		"selector": func(p *parser) *ast.Node {
			return p.Selector()
		},
	},
	"container": {
		"style": func(p *parser) *ast.Node {
			return p.Declaration()
		},
	},
}

// defaultConfig is the configuration of the full parser.
var defaultConfig = &config{
	contexts: map[string]contextFn{
		"default": func(p *parser, opts Options) *ast.Node {
			return p.StyleSheet()
		},
		"stylesheet": func(p *parser, opts Options) *ast.Node {
			return p.StyleSheet()
		},
		"atrule": func(p *parser, opts Options) *ast.Node {
			return p.Atrule(false)
		},
		"atrulePrelude": func(p *parser, opts Options) *ast.Node {
			var name jsutil.Str
			if len(opts.Atrule) != 0 {
				name = opts.Atrule
			}
			return p.AtrulePrelude(name)
		},
		"mediaQueryList": func(p *parser, opts Options) *ast.Node {
			return p.MediaQueryList()
		},
		"mediaQuery": func(p *parser, opts Options) *ast.Node {
			return p.MediaQuery()
		},
		"condition": func(p *parser, opts Options) *ast.Node {
			return p.Condition(opts.Kind)
		},
		"rule": func(p *parser, opts Options) *ast.Node {
			return p.Rule()
		},
		"selectorList": func(p *parser, opts Options) *ast.Node {
			return p.SelectorList()
		},
		"selector": func(p *parser, opts Options) *ast.Node {
			return p.Selector()
		},
		"block": func(p *parser, opts Options) *ast.Node {
			return p.Block(true)
		},
		"declarationList": func(p *parser, opts Options) *ast.Node {
			return p.DeclarationList()
		},
		"declaration": func(p *parser, opts Options) *ast.Node {
			return p.Declaration()
		},
		"value": func(p *parser, opts Options) *ast.Node {
			return p.Value()
		},
	},
	features: features,
	scope: scopeSet{
		AtrulePrelude: atrulePreludeScope,
		Selector:      selectorScope,
		Value:         valueScope,
	},
	atrule: atrules,
	pseudo: pseudos,
}

// selectorConfig is the configuration of the selector-only parser, which reads a selector list by
// default and knows nothing of at-rules or values.
var selectorConfig = &config{
	contexts: map[string]contextFn{
		"default": func(p *parser, opts Options) *ast.Node {
			return p.SelectorList()
		},
		"selectorList": func(p *parser, opts Options) *ast.Node {
			return p.SelectorList()
		},
		"selector": func(p *parser, opts Options) *ast.Node {
			return p.Selector()
		},
	},
	features: map[string]map[string]func(p *parser) *ast.Node{},
	scope: scopeSet{
		Selector: selectorScope,
	},
	atrule: map[string]atruleDef{},
	pseudo: pseudos,
}
