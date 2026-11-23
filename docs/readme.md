# API documentation

The Go API lives in the root package, `github.com/csstree/csstree`. CSS text is a `csstree.Str`,
a sequence of UTF-16 code units, because every offset the library records is a code unit offset;
`csstree.Text` turns a Go string into one and `(Str).String()` turns it back.

- [AST format](ast.md)
- [Parsing CSS → AST](parsing.md)
- [Serialization AST → CSS](generate.md)
- [AST traversal](traversal.md)
- [Util functions](utils.md)
- [The List type](list.md)
- [Working with definition syntax](definition-syntax.md)

Everything the API offers is also reachable from the command line; see the top of
`cmd/csstree/main.go` for its sub-commands.
