# Serialization AST to CSS

## Generate

```go
tree, _ := csstree.Parse(csstree.Text(".a { color: red }"), csstree.NewParseOptions())

css, err := csstree.Generate(tree, "")      // .a{color:red}
css, err = csstree.Generate(tree, "spec")   // the spec whitespace mode
```

The second argument is the whitespace mode. `""` and `"safe"` are the default mode, which puts a
whitespace between two tokens whenever leaving it out would make them read back as one token;
`"spec"` follows the specification's own rule, which inserts fewer. Any other value is the default
mode, as the original treated it.

A node type the generator does not know is an `*csstree.Error` named `Error` with the message
`Unknown node type: X`.

The CSS it writes is the shortest text that reads back as the same tree: no indentation, no
newlines and no comments, since comments are not part of the tree unless they were kept.
