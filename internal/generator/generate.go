// Package generator writes the CSS text of a tree. It walks a node, hands every token it reaches
// to a running buffer, and puts a whitespace between two tokens that would otherwise read back as
// one, following the table the mode picks.
package generator

import (
	"math"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

const reverseSolidus = 0x005c // U+005C REVERSE SOLIDUS (\)

var (
	spaceStr     = jsutil.FromString(" ")
	newlineStr   = jsutil.FromString("\n")
	undefinedStr = jsutil.FromString("undefined")
	nullStr      = jsutil.FromString("null")
	trueStr      = jsutil.FromString("true")
	falseStr     = jsutil.FromString("false")
	objectStr    = jsutil.FromString("[object Object]")
)

// generator carries the buffer a run writes into, the code of the token last written, and the
// table that decides whether a whitespace has to come before the next one.
type generator struct {
	buffer      jsutil.Str
	prevCode    int
	tokenBefore func(prevCode, tokenType int, value jsutil.Str) int

	// tokens collects what was written, each piece tagged with the node it came from, for the
	// lexer, which matches a tree against a syntax token by token and has to know which node
	// each token belongs to. It is nil for an ordinary run.
	tokens  []Token
	collect bool
	curNode *ast.Node
}

// Token is one piece of the written text: its token type, its text, and the node it came from,
// which is nil for a whitespace or a newline the generator added by itself.
type Token struct {
	Type  int
	Value jsutil.Str
	Node  *ast.Node
}

// Generate writes the CSS of a node. mode is "" or "safe" for the default mode and "spec" for the
// spec mode; any other value is the default mode, as the original treats it. It returns the error
// the original throws for a node type it does not know.
func Generate(node *ast.Node, mode string) (jsutil.Str, *jsutil.Error) {
	g := &generator{buffer: jsutil.Str{}, tokenBefore: safeTokenBefore}

	switch mode {
	case "spec":
		g.tokenBefore = specTokenBefore
	case "safe":
		g.tokenBefore = safeTokenBefore
	}

	if err := g.node(node); err != nil {
		return nil, err
	}

	return g.buffer, nil
}

// Tokens writes the CSS of a node and returns the pieces it wrote, each tagged with the node it
// came from. It is how the lexer turns a tree into the token list it matches against a syntax.
func Tokens(node *ast.Node) ([]Token, *jsutil.Error) {
	g := &generator{buffer: jsutil.Str{}, tokenBefore: safeTokenBefore, collect: true}

	if err := g.node(node); err != nil {
		return nil, err
	}

	return g.tokens, nil
}

// emit adds text to the buffer and, while tokens are being collected, records it against the
// node being written; a piece the generator added by itself belongs to no node.
func (g *generator) emit(value jsutil.Str, tokenType int, auto bool) {
	g.buffer = append(g.buffer, value...)

	if g.collect {
		node := g.curNode
		if auto {
			node = nil
		}
		g.tokens = append(g.tokens, Token{Type: tokenType, Value: value, Node: node})
	}
}

// token writes one token, putting a whitespace in front of it when the table asks for one and
// suppressAutoWhiteSpace does not forbid it, and a newline after a delim that is a backslash.
func (g *generator) token(tokenType int, value jsutil.Str, suppressAutoWhiteSpace bool) {
	g.prevCode = g.tokenBefore(g.prevCode, tokenType, value)

	if !suppressAutoWhiteSpace && g.prevCode&1 != 0 {
		g.emit(spaceStr, tokenizer.WhiteSpace, true)
	}

	g.emit(value, tokenType, false)

	if tokenType == tokenizer.Delim && value.CharCodeAt(0) == reverseSolidus {
		g.emit(newlineStr, tokenizer.WhiteSpace, true)
	}
}

// tokenize runs raw through the tokenizer and writes every token it reads. Only the token that
// starts the text may take an automatic whitespace in front of it.
func (g *generator) tokenize(raw jsutil.Str) {
	tokenizer.Tokenize(raw, func(tokenType, start, end int) {
		g.token(tokenType, raw.Substring(start, end), start != 0)
	})
}

// tokenizeField runs the value of a field through the tokenizer.
func (g *generator) tokenizeField(node *ast.Node, key string) {
	g.tokenize(stringField(node, key))
}

// node writes one node, or returns the error the original throws for a type it has no generator
// for.
func (g *generator) node(node *ast.Node) *jsutil.Error {
	if node == nil {
		return unknownNodeType(undefinedStr)
	}

	fn, ok := nodeGenerators[node.Type]
	if !ok {
		return unknownNodeType(jsutil.FromString(node.Type))
	}

	previous := g.curNode
	g.curNode = node
	err := fn(g, node)
	g.curNode = previous

	return err
}

// child writes the node a field holds. A field that holds anything else is reported the way the
// original reports it, because the type of a value that is not a node reads as undefined.
func (g *generator) child(node *ast.Node, key string) *jsutil.Error {
	if child, ok := node.Get(key).(*ast.Node); ok {
		return g.node(child)
	}
	return unknownNodeType(undefinedStr)
}

// children writes the children of a node. With a delimiter it calls it with the previous child
// before every child but the first.
func (g *generator) children(node *ast.Node, delimiter func(prev *ast.Node) *jsutil.Error) *jsutil.Error {
	var err *jsutil.Error

	if delimiter != nil {
		var prev *ast.Node

		node.List().Each(func(child *ast.Node, item *ast.Item, list *ast.List) {
			if err != nil {
				return
			}
			if prev != nil {
				if err = delimiter(prev); err != nil {
					return
				}
			}
			if err = g.node(child); err != nil {
				return
			}
			prev = child
		})

		return err
	}

	node.List().Each(func(child *ast.Node, item *ast.Item, list *ast.List) {
		if err != nil {
			return
		}
		err = g.node(child)
	})

	return err
}

// unknownNodeType is the error the original throws for a node it cannot write.
func unknownNodeType(typeName jsutil.Str) *jsutil.Error {
	return jsutil.NewError("Error", jsutil.Concat(jsutil.FromString("Unknown node type: "), typeName))
}

// stringField is the text of a field, the way JavaScript writes the value into a string: a field
// that is not there reads as undefined and a field that holds nothing reads as null.
func stringField(node *ast.Node, key string) jsutil.Str {
	if !node.Has(key) {
		return undefinedStr
	}
	return jsString(node.Get(key))
}

// jsString is String(value) in JavaScript for the values a field of a tree can hold.
func jsString(value any) jsutil.Str {
	switch v := value.(type) {
	case nil:
		return nullStr
	case undefinedField:
		return undefinedStr
	case jsutil.Str:
		return v
	case jsutil.JSONString:
		return jsutil.Str(v)
	case bool:
		if v {
			return trueStr
		}
		return falseStr
	case jsutil.Bool:
		if bool(v) {
			return trueStr
		}
		return falseStr
	case jsutil.Num:
		return jsutil.FromString(jsutil.NumberToString(float64(v)))
	case jsutil.Int:
		return jsutil.FromString(jsutil.NumberToString(float64(v)))
	case jsutil.Null:
		return nullStr
	case *ast.Node:
		if v == nil {
			return nullStr
		}
		return objectStr
	case *ast.List:
		if v == nil {
			return nullStr
		}
		return objectStr
	}
	return objectStr
}

// jsAdd is the JavaScript `+` of two field values: an addition when both are numbers and a text
// concatenation otherwise.
func jsAdd(left, right any) jsutil.Str {
	if a, ok := jsNumber(left); ok {
		if b, ok := jsNumber(right); ok {
			return jsutil.FromString(jsutil.NumberToString(a + b))
		}
	}
	return jsutil.Concat(jsString(left), jsString(right))
}

// jsNumber reads a value that is a number, and says whether it was one.
func jsNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case jsutil.Num:
		return float64(v), true
	case jsutil.Int:
		return float64(v), true
	}
	return 0, false
}

// truthy is the JavaScript truth of a field value.
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case undefinedField:
		return false
	case jsutil.Str:
		return len(v) > 0
	case jsutil.JSONString:
		return len(v) > 0
	case bool:
		return v
	case jsutil.Bool:
		return bool(v)
	case jsutil.Num:
		return float64(v) != 0 && !math.IsNaN(float64(v))
	case jsutil.Int:
		return int(v) != 0
	case jsutil.Null:
		return false
	case *ast.Node:
		return v != nil
	case *ast.List:
		return v != nil
	}
	return true
}

// fieldIsTruthy reports whether `node.<key>` passes an `if` in the original.
func fieldIsTruthy(node *ast.Node, key string) bool {
	return truthy(node.Get(key))
}

// fieldIsNull reports whether `node.<key> === null` holds. A field that is not there is undefined,
// which is not null, exactly as the original's strict comparison reads it.
func fieldIsNull(node *ast.Node, key string) bool {
	if !node.Has(key) {
		return false
	}

	switch v := node.Get(key).(type) {
	case nil:
		return true
	case jsutil.Null:
		return true
	case *ast.Node:
		return v == nil
	case *ast.List:
		return v == nil
	}

	return false
}

// strField is a field that holds a JavaScript string, and says whether it held one. The original
// reaches into a value with a string index or compares it with ===, which only a string answers.
func strField(node *ast.Node, key string) (jsutil.Str, bool) {
	switch v := node.Get(key).(type) {
	case jsutil.Str:
		return v, true
	case jsutil.JSONString:
		return jsutil.Str(v), true
	}
	return nil, false
}
