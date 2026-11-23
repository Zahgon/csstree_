# Parsing CSS into AST

> NOTE: The parser leaves out redundant separators, spaces and comments (except exclamation
> comments, i.e. `/*! comment */`) when it builds a tree.

## Parse

```go
import csstree "github.com/csstree/csstree"

// with the default options
tree, err := csstree.Parse(csstree.Text(".example { color: red }"), csstree.NewParseOptions())

// with options of your own
options := csstree.NewParseOptions()
options.Context = "selector"
options.Positions = true
tree, err = csstree.Parse(csstree.Text(".foo.bar"), options)
```

`Parse` returns the tree and, when the CSS cannot be read and no `OnParseError` was given, an
`*csstree.Error` named `SyntaxError` whose `Message` says what was expected and where.

`ParseSelector` is the same thing with the reduced node set of the selector-only parser; its
default context is a selector list.

## ParseOptions

Always build them with `NewParseOptions`: the zero value of the struct is not the default, because
three of the fields default to true and a position starts at line 1, column 1.

### Context

Which piece of CSS the source holds. The default is `stylesheet`.

| Context | Description |
| --- | --- |
| `stylesheet` (default) | Regular stylesheet, the most common use |
| `atrule` | At-rule, e.g. `@media screen, print { ... }` |
| `atrulePrelude` | At-rule prelude; set `Atrule` to the at-rule's name to read it the way that at-rule does |
| `mediaQueryList` | Media query list, e.g. `screen, print` |
| `mediaQuery` | Media query, e.g. `screen and (max-width: 100px)` |
| `condition` | Condition of a media, container or supports query; set `Kind` to which |
| `rule` | Rule, e.g. `.foo, .bar:hover { color: red; border: 1px solid black }` |
| `selectorList` | Selector list, e.g. `.foo, .bar:hover` |
| `selector` | Selector, e.g. `.foo.bar > a[href]` |
| `block` | Block of declarations, with the braces, e.g. `{ color: red }` |
| `declarationList` | Block content without the braces, e.g. `color: red; border: 1px solid` |
| `declaration` | Declaration, e.g. `color: red` |
| `value` | Declaration value, e.g. `red 1px solid` |

A context the parser does not know is an `*csstree.Error` named `Error`, with the message
`` Unknown context `name` ``.

### Positions

When set, every node carries a `Loc` with the file name and the start and end of the node, each as
an offset in UTF-16 code units, a line and a column. It is off by default and `Loc` is then `nil`.

### Filename, Offset, Line, Column

`Filename` is the name a location carries; without one it is `<unknown>`. `Offset`, `Line` and
`Column` shift the places the parser records, for CSS that was cut out of a larger file; their
defaults are 0, 1 and 1.

### ParseAtrulePrelude, ParseRulePrelude, ParseValue

All three default to true. Setting one to false leaves that part of the source as a `Raw` node
instead of reading it into a tree, which is faster when a tool does not look inside it.

### ParseCustomProperty

Off by default: the value of a custom property is left as a `Raw` node. Setting it reads the value
into a tree like any other.

### OnComment, OnToken, OnParseError

Callbacks. `OnComment` is told about every comment with its text and, when positions are on, where
it came from. `OnToken` is told about every token before the parse begins. `OnParseError` makes the
parse tolerant: instead of failing, the piece that could not be read becomes a `Raw` node and the
callback is told about the error and the node that took its place.

```go
options := csstree.NewParseOptions()
options.OnParseError = func(err *csstree.Error, fallback *csstree.Node) {
    fmt.Println(err.Message.String())
}
tree, _ := csstree.Parse(csstree.Text("a { color: }"), options)
```
