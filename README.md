<img align="right" width="111" height="111" alt="CSSTree logo" src="assets/csstree-logo-rounded.svg" />

# CSSTree

[![Build Status](https://github.com/csstree/csstree/actions/workflows/build.yml/badge.svg)](https://github.com/csstree/csstree/actions/workflows/build.yml)

CSSTree is a tool set for CSS: a detailed parser (CSS → AST), a walker (AST traversal), a
generator (AST → CSS) and a lexer (validation and matching) based on specs and browser
implementations. The main goal is to be efficient and W3C spec compliant, with focus on CSS
analyzing and source-to-source transforming tasks.

This is the Go implementation. It is one module with no dependencies outside the Go standard
library.

## Install

```
go get github.com/csstree/csstree
```

## Command line

The `csstree` command exposes every part of the tool set. Build it with:

```
go build ./cmd/csstree
```

Then, for example:

```
$ printf 'a{color:red}' | csstree roundtrip
a{color:red}

$ printf 'a{color:red}' | csstree parse | head -4
{
  "type": "StyleSheet",
  "loc": null,
  "children": [

$ csstree match --property color --value red
MATCHED
...

$ printf 'a{color:red}' | csstree tokenize
0	ident-token	0	1	6	"a"
1	{-token	1	2	5	"{"
...
```

The sub-commands are `tokenize`, `parse`, `parse-selector`, `generate`, `roundtrip`, `convert`,
`clone`, `walk`, `find`, `match`, `check`, `check-structure`, `syntax`, `defsyntax` and `util`.
Each one is documented at the top of `cmd/csstree/main.go`.

## Library

```go
package main

import (
	"fmt"

	csstree "github.com/csstree/csstree"
)

func main() {
	tree, err := csstree.Parse(csstree.Text("a { color: red }"), csstree.NewParseOptions())
	if err != nil {
		panic(err.Message.String())
	}

	// every declaration of the stylesheet
	csstree.Walk(tree, csstree.WalkOptions{
		Visit: "Declaration",
		Enter: func(node *csstree.Node, _ *csstree.Item, _ *csstree.List, _ *csstree.WalkContext) {
			fmt.Println(node.StrOrEmpty("property"))
		},
	})

	// back to CSS
	css, _ := csstree.Generate(tree, "")
	fmt.Println(css.String()) // a{color:red}

	// does the value match the property's grammar?
	lexer, _ := csstree.DefaultLexer()
	match := lexer.MatchProperty(csstree.Text("color"), csstree.TextValue(csstree.Text("red")))
	fmt.Println(match.Matched != nil) // true
}
```

CSS text is a `csstree.Str`, a sequence of UTF-16 code units, because every offset the library
records is a code unit offset. `csstree.Text` turns a Go string into one and `(Str).String()` turns
it back. The [API documentation](docs/readme.md) covers parsing, generating, traversal, the
definition syntax and the utilities.

## Features

- **Detailed parsing with an adjustable level of detail**

  By default CSSTree parses CSS as detailed as possible, i.e. each single logical part is
  represented by its own AST node (see [AST format](docs/ast.md) for all possible node types).
  The parsing detail level can be changed through parse options, for example, you can leave
  selectors or declaration values unparsed.

- **Tolerant to errors by design**

  The parser behaves as [the spec says](https://www.w3.org/TR/css-syntax-3/#error-handling):
  "When errors occur in CSS, the parser attempts to recover gracefully, throwing away only the
  minimum amount of content before returning to parsing as normal". The only thing the parser
  departs from the specification is that it does not throw away bad content, but wraps it in a
  special node type (`Raw`) that allows processing it later.

- **Syntax validation**

  The built-in lexer can test CSS against syntaxes defined by W3C. CSSTree uses
  [mdn/data](https://github.com/mdn/data/) as a basis for the lexer's dictionaries and extends it
  with vendor specific and legacy syntaxes. The data is carried in the repository under
  `data/mdn-data`, and `data/patch.json` holds the extensions; the dictionary is resolved from
  the two at run time, so the module needs no network and no package manager.

## Packages

| Package | What it does |
| :-- | :-- |
| `internal/tokenizer` | CSS Syntax Level 3 tokenizer, the token stream and offset-to-location |
| `internal/ast` | the syntax tree: nodes, lists, locations and the declared structure |
| `internal/parser` | CSS text to a tree, in every parse context, with error recovery |
| `internal/generator` | a tree back to CSS, in the safe and the spec whitespace modes |
| `internal/walker` | tree traversal with the walk context, and find/findLast/findAll |
| `internal/lexer` | matching values against the grammar, the checks and the structure check |
| `internal/syntaxdef` | the value definition syntax: parsing, generating and walking |
| `internal/dict` | the CSS syntax dictionary, resolved from the MDN data and the patch |
| `internal/cssutil` | string, url and identifier encoding and decoding, and name descriptors |
| `internal/jsutil` | the JavaScript semantics the results depend on: UTF-16 strings and JSON |

## Development

```
go vet ./...
go test ./...
gofmt -l .
```

## License

MIT
