// Package parser turns CSS text into a syntax tree.
//
// The entry points are Parse, which reads a whole style sheet or any smaller piece of CSS named
// by the context option, and ParseSelector, which reads a selector list with the reduced node set
// of the selector-only parser. Both take the same Options, which should be made with NewOptions so
// that the fields that default to something other than their zero value are set.
//
// The parser recovers from a syntax error wherever the original does: the piece that failed
// becomes a Raw node, and OnParseError, when it is given, is told about the error and the node
// that took the place of what failed. A syntax error that happens where there is nothing to
// recover into comes back from Parse as a *jsutil.Error named SyntaxError, whose Message is the
// full message with an excerpt of the source and the place marked. An OnParseError handler that
// panics with the *jsutil.Error it was given stops the recovery and makes Parse return that
// error, which is what a handler that rethrows does in the original.
package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

// Character codes the parser tests for by hand.
const (
	exclamationMark  = 0x0021 // !
	quotationMark    = 0x0022 // "
	numberSign       = 0x0023 // #
	dollarSign       = 0x0024 // $
	ampersand        = 0x0026 // &
	apostrophe       = 0x0027 // '
	leftParenthesis  = 0x0028 // (
	rightParenthesis = 0x0029 // )
	asterisk         = 0x002A // *
	plusSign         = 0x002B // +
	hyphenMinus      = 0x002D // -
	fullStop         = 0x002E // .
	solidus          = 0x002F // /
	colonChar        = 0x003A // :
	semicolon        = 0x003B // ;
	lessThanSign     = 0x003C // <
	equalsSign       = 0x003D // =
	greaterThanSign  = 0x003E // >
	questionMark     = 0x003F // ?
	letterN          = 0x006E // n
	letterU          = 0x0075 // u
	leftCurlyBracket = 0x007B // {
	verticalLine     = 0x007C // |
	tilde            = 0x007E // ~
	circumflexAccent = 0x005E // ^
	reverseSolidus   = 0x005C // \
)

// Options are the parse options of the original, with the same defaults once NewOptions has made
// them. The zero value of the struct is not the default: ParseValue, ParseAtrulePrelude and
// ParseRulePrelude are on by default and a position starts at line 1, column 1.
type Options struct {
	// Context names the piece of CSS the source holds. The empty string means the default
	// context, which is "stylesheet" for Parse and "selectorList" for ParseSelector.
	Context string

	// Positions says whether every node carries the place in the source it came from.
	Positions bool

	// Filename is the source name a location carries; nil means "<unknown>".
	Filename jsutil.Str

	// Offset, Line and Column shift the places the parser records, for CSS that was cut out of
	// a larger file. Their defaults are 0, 1 and 1.
	Offset int
	Line   int
	Column int

	// ParseValue, ParseAtrulePrelude and ParseRulePrelude read a declaration value, an at-rule
	// prelude and a rule prelude as a tree rather than leaving them as a Raw node.
	ParseValue         bool
	ParseAtrulePrelude bool
	ParseRulePrelude   bool

	// ParseCustomProperty reads the value of a custom property as a tree; it is off by default.
	ParseCustomProperty bool

	// OnComment is told about every comment in the source, with the text between the delimiters
	// and the place it came from, which is nil unless Positions is set.
	OnComment func(value jsutil.Str, loc *ast.Location)

	// OnToken is told about every token in the source before the parse starts.
	OnToken func(tokenType, start, end int)

	// OnParseError is told about every syntax error the parser recovered from, with the node
	// that took the place of the piece that failed.
	OnParseError func(err *jsutil.Error, fallbackNode *ast.Node)

	// Atrule is the at-rule name the atrulePrelude context reads a prelude for.
	Atrule jsutil.Str

	// Kind is the kind of condition the condition context reads.
	Kind jsutil.Str
}

// NewOptions returns the options the original parses with when it is given none.
func NewOptions() Options {
	return Options{
		ParseValue:         true,
		ParseAtrulePrelude: true,
		ParseRulePrelude:   true,
		Line:               1,
		Column:             1,
	}
}

// thrown is a JavaScript exception on its way up the stack. The original's parser reports every
// failure by throwing and catches them again in parseWithFallback, so the port raises them as a
// panic carrying this value and catches it in the same two places.
type thrown struct {
	err *jsutil.Error
}

// asThrown recovers the error a panic carries, or nil when the panic is not one of ours. An
// *jsutil.Error counts, so that an OnParseError handler can rethrow the error it was given.
func asThrown(value any) *thrown {
	switch v := value.(type) {
	case *thrown:
		return v
	case *jsutil.Error:
		return &thrown{err: v}
	}
	return nil
}

// typeError is the failure JavaScript raises when a parse step reaches for something that is not
// there, such as a condition kind that has no reader. The original lets parseWithFallback catch
// it like any other failure, so it travels the same way.
func typeError(message string) *thrown {
	return &thrown{err: jsutil.NewError("TypeError", jsutil.FromString(message))}
}

// parser reads a token stream into a tree. It is the one object of the original, whose methods
// are the node readers and whose own state is the stream's.
type parser struct {
	*tokenizer.TokenStream

	conf *config

	source        jsutil.Str
	filename      jsutil.Str
	needPositions bool

	locationMap *tokenizer.OffsetToLocation
	startOffset int
	startLine   int
	startColumn int

	onParseError      func(err *jsutil.Error, fallbackNode *ast.Node)
	onParseErrorThrow bool

	parseAtrulePrelude  bool
	parseRulePrelude    bool
	parseValue          bool
	parseCustomProperty bool
}

// Parse reads CSS text into a tree with the full set of node types.
func Parse(source jsutil.Str, opts Options) (*ast.Node, *jsutil.Error) {
	return newParser(defaultConfig).parse(source, opts)
}

// ParseSelector reads CSS text into a tree with the node types of the selector-only parser, whose
// default context is a selector list.
func ParseSelector(source jsutil.Str, opts Options) (*ast.Node, *jsutil.Error) {
	return newParser(selectorConfig).parse(source, opts)
}

func newParser(conf *config) *parser {
	return &parser{conf: conf}
}

func (p *parser) parse(source jsutil.Str, opts Options) (result *ast.Node, err *jsutil.Error) {
	p.source = source
	p.TokenStream = tokenizer.NewTokenStream(source, tokenizer.Tokenize)
	p.locationMap = tokenizer.NewOffsetToLocation(source, opts.Offset, opts.Line, opts.Column)
	p.startOffset = opts.Offset
	p.startLine = opts.Line
	p.startColumn = opts.Column

	p.filename = opts.Filename
	if len(p.filename) == 0 {
		p.filename = jsutil.FromString("<unknown>")
	}
	p.needPositions = opts.Positions
	p.onParseError = opts.OnParseError
	p.onParseErrorThrow = false

	p.parseAtrulePrelude = opts.ParseAtrulePrelude
	p.parseRulePrelude = opts.ParseRulePrelude
	p.parseValue = opts.ParseValue
	p.parseCustomProperty = opts.ParseCustomProperty

	context := opts.Context
	if context == "" {
		context = "default"
	}

	read, ok := p.conf.contexts[context]
	if !ok {
		return nil, jsutil.NewError("Error", jsutil.Concat(
			jsutil.FromString("Unknown context `"),
			jsutil.FromString(context),
			jsutil.FromString("`"),
		))
	}

	if opts.OnToken != nil {
		p.ForEachToken(func(tokenType, start, end, index int) {
			opts.OnToken(tokenType, start, end)
		})
	}

	if opts.OnComment != nil {
		p.ForEachToken(func(tokenType, start, end, index int) {
			if tokenType != tokenizer.Comment {
				return
			}
			loc := p.getLocation(start, end)
			var value jsutil.Str
			if tokenizer.CmpStr(p.source, end-2, end, "*/") {
				value = p.source.Substring(start+2, end-2)
			} else {
				value = p.source.Substring(start+2, end)
			}
			opts.OnComment(value, loc)
		})
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			caught := asThrown(recovered)
			if caught == nil {
				panic(recovered)
			}
			result = nil
			err = caught.err
		}
	}()

	tree := read(p, opts)

	if !p.EOF {
		p.fail(nil)
	}

	return tree, nil
}

// parseWithFallback reads with the consumer and, when it fails, winds the stream back to where it
// started, reads the fallback instead and reports the failure.
func parseWithFallback[T any](p *parser, consumer func() T, fallback func() T) T {
	startIndex := p.TokenIndex
	var result T

	caught := func() (caught *thrown) {
		defer func() {
			if recovered := recover(); recovered != nil {
				failure := asThrown(recovered)
				if failure == nil || p.onParseErrorThrow {
					panic(recovered)
				}
				caught = failure
			}
		}()
		result = consumer()
		return nil
	}()

	if caught == nil {
		return result
	}

	p.Skip(startIndex - p.TokenIndex)
	fallbackNode := fallback()

	p.onParseErrorThrow = true
	p.reportParseError(caught.err, any(fallbackNode))
	p.onParseErrorThrow = false

	return fallbackNode
}

// reportParseError hands a recovered failure to the handler. A fallback that is a list of nodes
// rather than one node reaches the handler as no node, since the handler takes a single node.
func (p *parser) reportParseError(err *jsutil.Error, fallback any) {
	if p.onParseError == nil {
		return
	}
	node, _ := fallback.(*ast.Node)
	p.onParseError(err, node)
}

// The four readers Raw is given to say where a run of raw tokens ends. A reader answers 0 to go
// on, 1 to stop before the token and 2 to stop after it.

func consumeUntilBalanceEnd(code uint16) int {
	return 0
}

func consumeUntilLeftCurlyBracket(code uint16) int {
	if code == leftCurlyBracket {
		return 1
	}
	return 0
}

func consumeUntilLeftCurlyBracketOrSemicolon(code uint16) int {
	if code == leftCurlyBracket || code == semicolon {
		return 1
	}
	return 0
}

func consumeUntilExclamationMarkOrSemicolon(code uint16) int {
	if code == exclamationMark || code == semicolon {
		return 1
	}
	return 0
}

func consumeUntilSemicolonIncluded(code uint16) int {
	if code == semicolon {
		return 2
	}
	return 0
}

// lookupNonWSType is the type of the first token at or after the offset that is neither white
// space nor a comment.
func (p *parser) lookupNonWSType(offset int) int {
	for {
		tokenType := p.LookupType(offset)
		offset++
		if tokenType != tokenizer.WhiteSpace && tokenType != tokenizer.Comment {
			return tokenType
		}
		if tokenType == tokenizer.EOF {
			return tokenizer.EOF
		}
	}
}

func (p *parser) charCodeAt(offset int) uint16 {
	if offset >= 0 && offset < len(p.source) {
		return p.source[offset]
	}
	return 0
}

func (p *parser) substring(start, end int) jsutil.Str {
	return p.source.Substring(start, end)
}

func (p *parser) cmpChar(offset int, code uint16) bool {
	return tokenizer.CmpChar(p.source, offset, code)
}

func (p *parser) cmpStr(start, end int, reference string) bool {
	return tokenizer.CmpStr(p.source, start, end, reference)
}

func (p *parser) consume(tokenType int) jsutil.Str {
	start := p.TokenStart
	p.eat(tokenType)
	return p.SubstrToCursor(start)
}

func (p *parser) consumeFunctionName() jsutil.Str {
	name := p.source.Substring(p.TokenStart, p.TokenEnd-1)
	p.eat(tokenizer.Function)
	return name
}

func (p *parser) consumeNumber(tokenType int) jsutil.Str {
	number := p.source.Substring(p.TokenStart, tokenizer.ConsumeNumber(p.source, p.TokenStart))
	p.eat(tokenType)
	return number
}

// tokenTypeName is the name the error message of eat calls a token type by: the token name
// without its "-token" ending, with hyphens turned into spaces and the first letter upper cased.
func tokenTypeName(tokenType int) string {
	name := tokenizer.Names[tokenType]
	name = name[:len(name)-len("-token")]
	letters := []byte(name)
	for i, c := range letters {
		if c == '-' {
			letters[i] = ' '
		}
	}
	if len(letters) > 0 && letters[0] >= 'a' && letters[0] <= 'z' {
		letters[0] -= 32
	}
	return string(letters)
}

func hasBracket(name string) bool {
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '[', ']', '(', ')', '{', '}':
			return true
		}
	}
	return false
}

func (p *parser) eat(tokenType int) {
	if p.TokenType != tokenType {
		name := tokenTypeName(tokenType)
		message := name + " is expected"
		if hasBracket(name) {
			message = "\"" + name + "\" is expected"
		}
		offset := p.TokenStart

		// tweak message and offset
		switch tokenType {
		case tokenizer.Ident:
			// when identifier is expected but there is a function or url
			if p.TokenType == tokenizer.Function || p.TokenType == tokenizer.Url {
				offset = p.TokenEnd - 1
				message = "Identifier is expected but function found"
			} else {
				message = "Identifier is expected"
			}

		case tokenizer.Hash:
			if p.IsDelim(numberSign, 0) {
				p.Next()
				offset++
				message = "Name is expected"
			}

		case tokenizer.Percentage:
			if p.TokenType == tokenizer.Number {
				offset = p.TokenEnd
				message = "Percent sign is expected"
			}
		}

		p.failAt(jsutil.FromString(message), offset)
	}

	p.Next()
}

func (p *parser) eatIdent(name string) {
	if p.TokenType != tokenizer.Ident || !p.LookupValue(0, name) {
		p.fail(jsutil.FromString("Identifier \"" + name + "\" is expected"))
	}

	p.Next()
}

func (p *parser) eatDelim(code uint16) {
	if !p.IsDelim(code, 0) {
		p.fail(jsutil.Concat(
			jsutil.FromString("Delim \""),
			jsutil.Str{code},
			jsutil.FromString("\" is expected"),
		))
	}

	p.Next()
}

func (p *parser) getLocation(start, end int) *ast.Location {
	if !p.needPositions {
		return nil
	}

	from, to := p.locationMap.GetLocationRange(start, end, p.filename)

	return &ast.Location{
		Source: from.Source,
		Start:  ast.Position{Offset: from.Offset, Line: from.Line, Column: from.Column},
		End:    ast.Position{Offset: to.Offset, Line: to.Line, Column: to.Column},
	}
}

func (p *parser) getLocationFromList(list *ast.List) *ast.Location {
	if !p.needPositions {
		return nil
	}

	start := p.TokenStart
	end := p.TokenStart

	if head := list.First(); head != nil {
		start = locOf(head).Start.Offset - p.startOffset
	}
	if tail := list.Last(); tail != nil {
		end = locOf(tail).End.Offset - p.startOffset
	}

	from, to := p.locationMap.GetLocationRange(start, end, p.filename)

	return &ast.Location{
		Source: from.Source,
		Start:  ast.Position{Offset: from.Offset, Line: from.Line, Column: from.Column},
		End:    ast.Position{Offset: to.Offset, Line: to.Line, Column: to.Column},
	}
}

// locOf reads the place a child came from, which a tree built with positions always has.
func locOf(node *ast.Node) *ast.Location {
	if node.Loc == nil {
		panic(typeError("Cannot read properties of null (reading 'start')"))
	}
	return node.Loc
}

// fail raises a syntax error at the current token.
func (p *parser) fail(message jsutil.Str) {
	p.raise(message, 0, false)
}

// failAt raises a syntax error at an offset of its own.
func (p *parser) failAt(message jsutil.Str, offset int) {
	p.raise(message, offset, true)
}

func (p *parser) raise(message jsutil.Str, offset int, hasOffset bool) {
	var location tokenizer.Loc

	switch {
	case hasOffset && offset < len(p.source):
		location = p.locationMap.GetLocation(offset, nil)
	case p.EOF:
		location = p.locationMap.GetLocation(tokenizer.FindWhiteSpaceStart(p.source, len(p.source)-1), nil)
	default:
		location = p.locationMap.GetLocation(p.TokenStart, nil)
	}

	if len(message) == 0 {
		message = jsutil.FromString("Unexpected input")
	}

	panic(&thrown{err: newSyntaxError(
		message,
		p.source,
		location.Offset,
		location.Line,
		location.Column,
		p.startLine,
		p.startColumn,
	)})
}

// Values a node field holds. A field that is null in the original is set to an untyped nil, which
// is what writes out as null; a Go nil of a pointer type would not.

func nodeOrNull(node *ast.Node) any {
	if node == nil {
		return nil
	}
	return node
}

func listOrNull(list *ast.List) any {
	if list == nil {
		return nil
	}
	return list
}

func strOrNull(value jsutil.Str) any {
	if value == nil {
		return nil
	}
	return value
}
