# Value Definition Syntax

This article describes API to work with [Value Definition Syntax](https://www.w3.org/TR/css-values-4/#value-defs) (or CSS Definition Syntax since it used to define various parts of CSS language beside values). API provides ability to parse a definition into AST, traverse through it and translate AST back to a string (see corresponding section for details).

<img width="560" alt="Example of definition syntax" src="https://user-images.githubusercontent.com/270491/66405190-68231d80-e9f2-11e9-8a3f-ea5e41e72991.png">

<!-- TOC depthFrom:2 -->

- [parse(source)](#parsesource)
- [walk(node, options, context)](#walknode-options-context)
- [generate(node, options)](#generatenode-options)
- [AST format](#ast-format)
    - [AtKeyword](#atkeyword)
    - [Comma](#comma)
    - [Function](#function)
    - [Group](#group)
    - [Keyword](#keyword)
    - [Multiplier](#multiplier)
    - [Property](#property)
    - [Range](#range)
    - [String](#string)
    - [Token](#token)
    - [Type](#type)

<!-- /TOC -->

## parse(source)

Arguments:

- **source**: `string`  
  A definition to parse

```go
import csstree "github.com/csstree/csstree"

definition, err := csstree.ParseDefinition(csstree.Text("foo | bar"))
// a Group whose terms are the keywords foo and bar,
// with combinator "|", disallowEmpty false and explicit false
```

## walk(node, options, context)

Arguments:

- **node**: `Object`  
  AST node
- **options**: `{ enter?: function, leave?: function }` or `function`  
  An object to specify enter and/or leave handlers. When value is a function, it treated as `{ enter: function }`. One of handlers is required.
- **context** (optional): `any`  
  Defines a value as `this` in enter and leave handlers.

```go
definition, _ := csstree.ParseDefinition(csstree.Text("foo | bar"))

csstree.WalkDefinition(definition, func(node *csstree.Definition) {
    fmt.Println("enter", node.Type)
}, func(node *csstree.Definition) {
    fmt.Println("leave", node.Type)
    }
});
// enter Group |
// enter Keyword foo
// leave Keyword foo
// enter Keyword bar
// leave Keyword bar
// leave Group |

definitionSyntax.walk(ast, node =>
    console.log(node.type, node.combinator || node.name || '')
);
// Group |
// Keyword foo
// Keyword bar
```

## generate(node, options)

- **node**: `Object`  
  AST node to generate a string from
- **options** (optional): `Object`  
  An object to specify output behaviour (all options are optional):
  - **forceBraces**: `Boolean` (default: `false`)  
    Enforce printing brackets for any groups (even implicit). Useful for debugging and priority revelation.
  - **compact**: `Boolean` (default: `false`)  
    Avoid formatting (primary whitespaces around brackets and so on) when possible.
  - **decorate**: `function(nodeGenerateResult, node)`  
    A function to post-process result of node translation to a string. Handy to make some kind of result wrapping.

```go
definition, _ := csstree.ParseDefinition(csstree.Text("foo && bar || [ baz | qux ]"))

text, _ := csstree.GenerateDefinition(definition, false, false)
// "foo && bar || [ baz | qux ]"

compact, _ := csstree.GenerateDefinition(definition, false, true)
// "foo&&bar||[baz|qux]"

definitionSyntax.generate(ast, { forceBraces: true });
// "[ [ foo && bar ] || [ baz | qux ] ]"

definitionSyntax.generate(ast, {
    decorate(content, node) {
        return node.type === 'Keyword' && node.name.startsWith('b')
            ? `<span class="spotlight">${content}</span>`
            : content;
    }
});
// "foo && <span class="spotlight">bar</span> || [ <span class="spotlight">baz</span> | qux ]"
```

## AST format

### AtKeyword

```go
{
    Type: "AtKeyword"
    Name: Str
}
```

A CSS at-keyword token (a keyword prescended by commertial at), e.g. `@media` or `@keyframes`.

## Reading a definition node

A parsed definition is a `*csstree.Definition`. Its `Type` says which of the shapes below it is,
and its other fields are read by name, because a node carries exactly the fields its own shape
calls for and in the order they were set:

```go
definition, _ := csstree.ParseDefinition(csstree.Text("<length>#{1,4}"))

definition.Type                 // "Multiplier"
definition.Num("min")           // 1
definition.Num("max")           // 4
definition.Bool("comma")        // true
definition.Child("term").Type   // "Type"
definition.Terms("terms")       // the terms of a Group
definition.Str("name")          // the name of a Type, Keyword, Property or Function
```

The shapes below name each field with the type of value it holds.

### Comma

```go
{
    Type: "Comma"
}
```

Just a comma. Comma is widely used and has complicated rules in definition syntax, that's why it represents by a separate node type.

### Function

```go
{
    Type: "Function"
    Name: Str
}
```

A CSS function token (a keyword followed by left parenthesis), e.g. `foo(` or `rgb(`.

### Group

```go
{
    Type: "Group"
    Terms: Array.<Object>
    Combinator: Str
    DisallowEmpty: bool
    Explicit: bool
}
```

A conjunction of `terms` joined by a `combinator`.

- **combinator** is enum value of ` ` (single space), `|`, `||` or `&&` (see [Component Value Combinators](https://drafts.csswg.org/css-values-4/#component-combinators) in spec).
- **disallowEmpty** is using to express `[ ]!` group (indicates that the group is required and must produce at least one value), only for such groups this field has true value.
- **explicit**, when `true`, means brackets was explicitly used for a group. Implicit groups is producing when mixed combinators are used for a term sequence, e.g. `a && b c || d` looks like `[ [ a && [ b c ] ] || d ]` when brackets are used for each group. The field mostly used to restore an original definition from AST.

### Keyword

```go
{
    Type: "Keyword"
    Name: Str
}
```

A keyword value (such as `auto`, `green`, `none` etc) which appear literally.

### Multiplier

```go
{
    Type: "Multiplier"
    Comma: bool
    Min: float64
    Max: float64
    Term: Object
}
```

A multipler of a term.

| Definition | AST | {N,M} notation
| ---------- | ---------- | --------------
| ? | { comma: false, min: 0, max: 1 } | {0,1}
| * | { comma: false, min: 0, max: 0 } | {0,}
| + | { comma: false, min: 1, max: 0 } | {1,}
| # | { comma: true, min: 1, max: 0 } | –
| #? | { comma: true, min: 0, max: 0 } | –
| {3} | { comma: false, min: 3, max: 3 } | {3}
| {3,} | { comma: false, min: 3, max: 0 } | {3,}
| {3,3} | { comma: false, min: 3, max: 3 } | {3}
| {3,6} | { comma: false, min: 3, max: 6 } | {3,6}
| #{3} | { comma: true, min: 3, max: 3 } | –
| #{3,} | { comma: true, min: 3, max: 0 } | –
| #{3,6} | { comma: true, min: 3, max: 6 } | –

> [!NOTE]
> As per spec [some multipliers may stack](https://drafts.csswg.org/css-values-4/#component-combinators) (e.g. `+#`, `{A}?`, `{A,B}?`), in this case `term` field of `Multiplier` node will contain another `Multiplier` node. For example, `+#` is represented as following AST:
> ```
> { ...<multiplier #>,
>   term: {
>     ...<multipler +>,
>     term: node
>   }
> }
> ```

### Property

```go
{
    Type: "Property"
    Name: Str
}
```

A reference to property definition, e.g. `<'property'>`.

### Range

```go
{
    Type: "Range"
    Min: Number | null
    Max: Number | null
}
```

Represents [bracketed range notation](https://drafts.csswg.org/css-values-4/#numeric-ranges). For example, in definition `<length [0,∞]>` the bracketed range notation is `[0,∞]`. `null` value is used for `min` and `max` to express `-∞` (-infinity) and `∞` (infinity) respectively.

<table>
<thead><tr>
<th>Definition
<th>AST
</thead>
<tbody>
<tr>
<td>&lt;length>
<td><pre>
{
    type: 'Type',
    name: 'length',
    opts: null
}
</pre></tr>
<tr>
<td>&lt;length [0,∞]>
<td><pre>
{
    type: 'Type',
    name: 'length',
    opts: {
        type: 'Range',
        min: 0,
        max: null
    }
}
</pre></tr>
<tr>
<td>&lt;length [-∞,10]>
<td><pre>
{
    type: 'Type',
    name: 'length',
    opts: {
        type: 'Range',
        min: null,
        max: 10
    }
}
</pre></tr>
</table>

### String

```go
{
    Type: "String"
    Value: Str
}
```

A sequence of characters.

### Token

```go
{
    Type: "Token"
    Value: Str
}
```

Any single code point isn't consumed by any other production excluding whitespaces and symbols used for multipliers, i.e. `*` (asterisk),
`+` (plus sign), `?` (question mark), `#` (number sign) and `!` (exclamation mark).

### Type

```go
{
    Type: "Type"
    Name: Str
    Opts: Object | null
}
```

Represents a reference to a type, e.g. `<length>`. `opts` is used to express something special like bracketed range notation (see [Range](#range) node type)
