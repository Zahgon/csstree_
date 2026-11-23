// Package csstree is a tool set for CSS: a tokenizer, a parser that builds a syntax tree, a
// generator that writes CSS back out, a walker, and a lexer that matches values against the CSS
// grammar.
//
// The CSS syntax dictionary the lexer resolves is built from the MDN CSS data the repository
// carries under data/mdn-data together with data/patch.json, the same two sources the original
// JavaScript implementation read it from.
package csstree

import (
	"embed"
	"fmt"
	"sync"

	"github.com/csstree/csstree/internal/dict"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/lexer"
)

//go:embed data/patch.json data/mdn-data/at-rules.json data/mdn-data/properties.json data/mdn-data/syntaxes.json
var dataFiles embed.FS

var (
	once        sync.Once
	sharedDict  *dict.Dict
	sharedLexer *lexer.Lexer
	buildErr    error
)

func build() {
	read := func(name string) (jsutil.Value, error) {
		raw, err := dataFiles.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		value, err := jsutil.ParseJSONBytes(raw)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		return value, nil
	}

	atrules, err := read("data/mdn-data/at-rules.json")
	if err != nil {
		buildErr = err
		return
	}
	properties, err := read("data/mdn-data/properties.json")
	if err != nil {
		buildErr = err
		return
	}
	syntaxes, err := read("data/mdn-data/syntaxes.json")
	if err != nil {
		buildErr = err
		return
	}
	patch, err := read("data/patch.json")
	if err != nil {
		buildErr = err
		return
	}

	sharedDict, buildErr = dict.Build(atrules, properties, syntaxes, patch)
	if buildErr != nil {
		return
	}

	sharedLexer = lexer.New(sharedDict)
}

// Dictionary is the resolved CSS syntax dictionary.
func Dictionary() (*Dict, error) {
	once.Do(build)
	return sharedDict, buildErr
}

// DefaultLexer is the lexer over the default dictionary.
func DefaultLexer() (*Lexer, error) {
	once.Do(build)
	return sharedLexer, buildErr
}
