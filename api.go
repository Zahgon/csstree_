package csstree

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/cssutil"
	"github.com/csstree/csstree/internal/dict"
	"github.com/csstree/csstree/internal/generator"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/lexer"
	"github.com/csstree/csstree/internal/parser"
	"github.com/csstree/csstree/internal/syntaxdef"
	"github.com/csstree/csstree/internal/tokenizer"
	"github.com/csstree/csstree/internal/walker"
)

// The types the tool set works with. CSS text is a sequence of UTF-16 code units, because every
// offset a tree records is a code unit offset, so Str rather than string is what the parser,
// the generator and the lexer take and give back.
type (
	// Str is CSS text: a sequence of UTF-16 code units.
	Str = jsutil.Str

	// Error is a failure the library reports: a syntax error in CSS text or in a value
	// definition, a name the dictionary does not know, or a value that does not match its syntax.
	Error = jsutil.Error

	// Node is one node of the syntax tree.
	Node = ast.Node

	// List is the list a node holds its children in, and Item one place in such a list.
	List = ast.List
	Item = ast.Item

	// Location is the span of source a node came from, and Position one point in it.
	Location = ast.Location
	Position = ast.Position

	// ParseOptions are the options a parse takes; NewParseOptions gives their defaults.
	ParseOptions = parser.Options

	// WalkOptions are the options a walk takes, and WalkContext the enclosing nodes a callback
	// is told about.
	WalkOptions = walker.Options
	WalkContext = walker.Context

	// Lexer matches values against the CSS grammar.
	Lexer = lexer.Lexer

	// MatchResult is what a match left behind: the syntaxes the value matched, or the error that
	// says why it did not.
	MatchResult = lexer.MatchResult

	// Value is a value to match, either as CSS text or as a tree.
	Value = lexer.Value

	// Dict is the resolved CSS syntax dictionary.
	Dict = dict.Dict

	// Definition is one node of a parsed value definition syntax.
	Definition = syntaxdef.Node

	// JSONValue is a tree in its plain form, ready to be written as JSON.
	JSONValue = jsutil.Value
)

// Text turns a Go string into CSS text.
func Text(s string) Str { return jsutil.FromString(s) }

// TextFromBytes reads CSS text from UTF-8 bytes, replacing what is not well formed, as a
// JavaScript runtime does when it reads a file.
func TextFromBytes(b []byte) Str { return jsutil.FromUTF8(b) }

// NewParseOptions returns the parse options with their defaults: declaration values, at-rule
// preludes and rule preludes are read as trees, custom property values are not, and a position
// starts at line 1, column 1.
func NewParseOptions() ParseOptions { return parser.NewOptions() }

// Parse reads CSS text into a tree.
func Parse(source Str, opts ParseOptions) (*Node, *Error) { return parser.Parse(source, opts) }

// ParseSelector reads CSS text with the selector-only parser, whose default context is a selector
// list.
func ParseSelector(source Str, opts ParseOptions) (*Node, *Error) {
	return parser.ParseSelector(source, opts)
}

// Tokenize hands every token of the CSS text to onToken, with its type and the half-open range of
// code units it covers.
func Tokenize(source Str, onToken func(tokenType, start, end int)) {
	tokenizer.Tokenize(source, onToken)
}

// TokenName is the CSS Syntax name of a token type, such as "ident-token".
func TokenName(tokenType int) string { return tokenizer.Names[tokenType] }

// NewTokenStream reads the whole text into a stream that can be walked and looked ahead in.
func NewTokenStream(source Str) *tokenizer.TokenStream {
	return tokenizer.NewTokenStream(source, tokenizer.Tokenize)
}

// Generate writes the CSS of a tree. mode is "" or "safe" for the default whitespace mode and
// "spec" for the spec mode.
func Generate(node *Node, mode string) (Str, *Error) { return generator.Generate(node, mode) }

// Walk visits the tree, telling the callbacks of WalkOptions about every node it reaches.
func Walk(root *Node, opts WalkOptions) *Error { return walker.Walk(root, opts) }

// Find, FindLast and FindAll search the tree for the nodes a test accepts.
func Find(root *Node, fn walker.Predicate) *Node      { return walker.Find(root, fn) }
func FindLast(root *Node, fn walker.Predicate) *Node  { return walker.FindLast(root, fn) }
func FindAll(root *Node, fn walker.Predicate) []*Node { return walker.FindAll(root, fn) }

// Clone copies a tree.
func Clone(node *Node) *Node { return ast.Clone(node) }

// ToPlainObject turns a tree into the plain value a JSON writer takes, and FromPlainObject turns
// one back into a tree.
func ToPlainObject(node *Node) JSONValue { return ast.ToPlain(node) }
func FromPlainObject(v JSONValue) *Node  { return ast.FromPlain(v) }

// JSON writes a plain value as JSON text, with the given indent; an indent of 0 writes it without
// any whitespace.
func JSON(v JSONValue, indent int) Str { return jsutil.Stringify(v, indent) }

// ParseJSON reads JSON text into a plain value, keeping the order its keys were written in.
func ParseJSON(text Str) (JSONValue, error) { return jsutil.ParseJSON(text) }

// TextValue wraps CSS text for the lexer to match, and NodeValue wraps a tree.
func TextValue(text Str) Value   { return lexer.TextValue(text) }
func NodeValue(node *Node) Value { return lexer.NodeValue(node) }

// ParseDefinition reads a value definition syntax.
func ParseDefinition(source Str) (*Definition, *Error) { return syntaxdef.Parse(source) }

// GenerateDefinition writes a value definition syntax back out. A definition tree that was not
// built by ParseDefinition may hold a node the writer does not know; that is reported as an error
// rather than left to unwind.
func GenerateDefinition(node *Definition, forceBraces, compact bool) (text Str, err *Error) {
	defer func() { text, err = recovered(recover(), text) }()
	return syntaxdef.Generate(node, forceBraces, compact), nil
}

// WalkDefinition visits a parsed value definition syntax, reporting a node the walk does not know
// as an error rather than leaving it to unwind.
func WalkDefinition(node *Definition, enter, leave func(*Definition)) (err *Error) {
	defer func() { _, err = recovered(recover(), nil) }()
	syntaxdef.Walk(node, enter, leave)
	return nil
}

// recovered turns a value the library raised while unwinding into an error, and raises anything
// else again, since a failure that is not one of ours is not ours to swallow.
func recovered(value any, text Str) (Str, *Error) {
	if value == nil {
		return text, nil
	}
	raised, ok := value.(*Error)
	if !ok {
		panic(value)
	}
	return nil, raised
}

// The string, url and identifier encoding and decoding of CSS text.
func DecodeString(s Str) Str                  { return cssutil.DecodeString(s) }
func EncodeString(s Str, apostrophe bool) Str { return cssutil.EncodeString(s, apostrophe) }
func DecodeURL(s Str) Str                     { return cssutil.DecodeURL(s) }
func EncodeURL(s Str) Str                     { return cssutil.EncodeURL(s) }
func DecodeIdent(s Str) Str                   { return cssutil.DecodeIdent(s) }
func EncodeIdent(s Str) Str                   { return cssutil.EncodeIdent(s) }

// Keyword reads a keyword into its parts: the lower cased name, the vendor prefix, and the name
// without that prefix.
func Keyword(name Str) cssutil.KeywordDescriptor { return cssutil.Keyword(name) }

// Property reads a property name into its parts: the hack it may begin with, the vendor prefix,
// the name without either, and whether it is a custom property.
func Property(name Str) cssutil.PropertyDescriptor { return cssutil.Property(name) }

// IsCustomProperty reports whether a name, read from the offset on, begins with two hyphens.
func IsCustomProperty(name Str, offset int) bool { return cssutil.IsCustomProperty(name, offset) }

// VendorPrefix is the vendor prefix a name carries from the offset on, or an empty string.
func VendorPrefix(name Str, offset int) Str { return cssutil.VendorPrefix(name, offset) }
