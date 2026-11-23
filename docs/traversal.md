# AST traversal

## Walk

```go
tree, _ := csstree.Parse(csstree.Text("@media print { .a { color: red } }"), csstree.NewParseOptions())

csstree.Walk(tree, csstree.WalkOptions{
    Enter: func(node *csstree.Node, item *csstree.Item, list *csstree.List, ctx *csstree.WalkContext) {
        fmt.Println(node.Type)
    },
})
```

`Walk` returns an `*csstree.Error` only when the options ask for something it cannot do; the walk
itself does not fail.

### WalkOptions

| Field | What it does |
| --- | --- |
| `Enter` | Called when a node is reached, before its children |
| `Leave` | Called when a node is left, after its children |
| `Visit` | A node type; the callbacks are then told only about nodes of that type |
| `Reverse` | Walk the children of every node from the last to the first |

A `Visit` naming a type that does not exist is an `*csstree.Error` named `Error`, whose message
lists every node type there is.

### What a callback is given

`node` is the node reached. `item` and `list` are the place it sits in, when it sits in a list of
children, and `nil` when it does not — together they let a callback remove the node or put another
one beside it while the walk goes on. `ctx` is the enclosing nodes:

| Field | The nearest enclosing |
| --- | --- |
| `Root` | node the walk started from |
| `StyleSheet` | `StyleSheet` |
| `Atrule` | `Atrule` |
| `AtrulePrelude` | `AtrulePrelude` |
| `Rule` | `Rule` |
| `Selector` | `SelectorList` |
| `Block` | `Block` |
| `Declaration` | `Declaration` |
| `Function` | `Function`, `PseudoClassSelector` or `PseudoElementSelector` |

Each is `nil` until the walk is inside such a node, and `nil` again once it has left.

## Find, FindLast and FindAll

```go
declarations := csstree.FindAll(tree, func(node *csstree.Node, _ *csstree.Item, _ *csstree.List, _ *csstree.WalkContext) bool {
    return node.Type == "Declaration"
})

first := csstree.Find(tree, isDeclaration)      // the first match, or nil
last := csstree.FindLast(tree, isDeclaration)   // the last match, or nil
```

They walk the tree in the same order and hand the test the same four values a walk callback gets,
so a test can ask where a node sits as well as what it is.
