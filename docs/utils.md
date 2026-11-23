# Util functions

## Strings, urls and identifiers

CSS text escapes things, and these turn the written form into the value it stands for and back.

```go
csstree.DecodeString(csstree.Text(`"a\62 c"`))      // abc
csstree.EncodeString(csstree.Text(`a"b`), false)    // "a\"b"
csstree.EncodeString(csstree.Text("a'b"), true)     // 'a\'b'   (apostrophe quoting)

csstree.DecodeURL(csstree.Text("url( a )"))         // a
csstree.EncodeURL(csstree.Text("a b"))              // url(a\ b)

csstree.DecodeIdent(csstree.Text(`a\62 c`))         // abc
csstree.EncodeIdent(csstree.Text("1a"))             // \31 a
```

`EncodeString`, `EncodeURL` and `EncodeIdent` follow § 2.1 of CSSOM, so what they write reads back
as the value they were given: a NULL becomes the replacement character, a control character becomes
a hexadecimal escape with a space after it where one is needed, and a character that would end the
token is escaped with a backslash.

## Names

```go
keyword := csstree.Keyword(csstree.Text("-WEBKIT-Foo"))
// Name "-webkit-foo", Vendor "-webkit-", Basename "foo", Custom false

property := csstree.Property(csstree.Text("*-moz-Box"))
// Hack "*", Name "-moz-box", Vendor "-moz-", Prefix "*-moz-", Basename "box", Custom false

csstree.IsCustomProperty(csstree.Text("--x"), 0)    // true
csstree.VendorPrefix(csstree.Text("-webkit-a"), 0)  // -webkit-
```

A keyword is lower cased; a property name is lower cased unless it is a custom property, whose
case is kept because a custom property name is case sensitive. `Hack` is the character a stylesheet
may put in front of a property name to hide it from some browsers (`*`, `_`, `$`, `#`, `+`, `&`,
`/` or `//`).

## Tokens

```go
csstree.Tokenize(csstree.Text("a{}"), func(tokenType, start, end int) {
    fmt.Println(csstree.TokenName(tokenType), start, end)
})

stream := csstree.NewTokenStream(csstree.Text("a{b:c}"))
for stream.TokenType != 0 { // 0 is the EOF token
    fmt.Println(csstree.TokenName(stream.TokenType), stream.TokenStart, stream.TokenEnd)
    stream.Next()
}
```

`Tokenize` hands every token to the callback as it reads it. A `TokenStream` reads the whole text
first, which lets it be walked, looked ahead in, and asked which token closes the block another
one opens.

## JSON

```go
plain := csstree.ToPlainObject(tree)
text := csstree.JSON(plain, 2)              // the tree as indented JSON
back := csstree.FromPlainObject(plain)      // and back into a tree

value, err := csstree.ParseJSON(csstree.Text(`{"b":1,"a":2}`))
```

`JSON` writes the keys in the order they were set, which for a node is `type`, `loc` and then the
node's own fields in the order the parser set them. `ParseJSON` keeps that order too.
