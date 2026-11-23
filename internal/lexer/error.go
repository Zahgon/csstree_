package lexer

import (
	"strings"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/syntaxdef"
)

// defaultLoc is where a mismatch is said to be when nothing in the value says otherwise.
var defaultLoc = ast.Position{Offset: 0, Line: 1, Column: 1}

// newSyntaxReferenceError reports a name the dictionary does not know.
func newSyntaxReferenceError(kind string, referenceName jsutil.Str) *jsutil.Error {
	message := jsutil.FromString(kind)
	if len(referenceName) > 0 {
		message = jsutil.Concat(message, jsutil.FromString(" `"), referenceName, jsutil.FromString("`"))
	}

	err := jsutil.NewError("SyntaxReferenceError", message)
	return err
}

// mismatchPlace is where a value stopped matching its syntax.
type mismatchPlace struct {
	css            jsutil.Str
	mismatchOffset int
	mismatchLength int
	start          ast.Position
	end            ast.Position
}

// locateMismatch walks the tokens of a value to find where it stopped matching, and where in the
// source that is.
func locateMismatch(result *internalResult, value Value) mismatchPlace {
	tokens := result.tokens
	longestMatch := result.longestMatch

	var mismatchNode *ast.Node
	if longestMatch < len(tokens) {
		mismatchNode = tokens[longestMatch].Node
	}

	var node *ast.Node
	if !value.IsText {
		node = value.Node
	}

	badNode := mismatchNode
	if mismatchNode == node {
		badNode = nil
	}

	mismatchOffset := 0
	mismatchLength := 0
	entries := 0
	css := jsutil.Str{}

	for i := 0; i < len(tokens); i++ {
		token := tokens[i].Value

		if i == longestMatch {
			mismatchLength = len(token)
			mismatchOffset = len(css)
		}

		if badNode != nil && tokens[i].Node == badNode {
			if i <= longestMatch {
				entries++
			} else {
				entries = 0
			}
		}

		css = append(css, token...)
	}

	var start, end ast.Position

	if longestMatch == len(tokens) || entries > 1 {
		// last
		source := badNode
		if source == nil {
			source = node
		}
		if place, ok := fromLoc(source, false); ok {
			start = place
		} else {
			start = buildLoc(defaultLoc, css)
		}
		end = start
	} else {
		if place, ok := fromLoc(badNode, true); ok {
			start = place
		} else {
			base := defaultLoc
			if place, ok := fromLoc(node, true); ok {
				base = place
			}
			start = buildLoc(base, css.Substring(0, mismatchOffset))
		}

		if place, ok := fromLoc(badNode, false); ok {
			end = place
		} else {
			end = buildLoc(start, css.Substring(mismatchOffset, mismatchOffset+mismatchLength))
		}
	}

	return mismatchPlace{css: css, mismatchOffset: mismatchOffset, mismatchLength: mismatchLength, start: start, end: end}
}

// fromLoc reads the start or the end of the place a node came from.
func fromLoc(node *ast.Node, start bool) (ast.Position, bool) {
	if node == nil || node.Loc == nil {
		return ast.Position{}, false
	}
	if start {
		return node.Loc.Start, true
	}
	return node.Loc.End, true
}

// buildLoc moves a place forward over a piece of text.
func buildLoc(from ast.Position, extra jsutil.Str) ast.Position {
	loc := from

	if len(extra) == 0 {
		return loc
	}

	lines := splitNewlines(extra)

	loc.Offset += len(extra)
	loc.Line += len(lines) - 1
	if len(lines) == 1 {
		loc.Column += len(extra)
	} else {
		loc.Column = len(lines[len(lines)-1]) + 1
	}

	return loc
}

// splitNewlines cuts a piece of text at every newline, the way the original's expression does.
func splitNewlines(text jsutil.Str) []jsutil.Str {
	lines := []jsutil.Str{}
	start := 0

	for i := 0; i < len(text); i++ {
		code := text[i]
		if code == 0x000A || code == 0x000C {
			lines = append(lines, text.Substring(start, i))
			start = i + 1
			continue
		}
		if code == 0x000D {
			lines = append(lines, text.Substring(start, i))
			if i+1 < len(text) && text[i+1] == 0x000A {
				i++
			}
			start = i + 1
		}
	}

	return append(lines, text.Slice(start))
}

// newSyntaxMatchError reports a value that does not match its syntax, with the syntax, the value
// and a marker under the place it stopped matching.
func newSyntaxMatchError(message string, syntax *syntaxdef.Node, value Value, result *internalResult) *jsutil.Error {
	place := locateMismatch(result, value)

	syntaxText := jsutil.FromString("<generic>")
	if syntax != nil {
		syntaxText = syntaxdef.Generate(syntax, false, false)
	}

	cssText := place.css
	if len(cssText) == 0 {
		cssText = jsutil.FromString("<empty string>")
	}

	full := jsutil.Concat(
		jsutil.FromString(message),
		jsutil.FromString("\n  syntax: "), syntaxText,
		jsutil.FromString("\n   value: "), cssText,
		jsutil.FromString("\n  --------"), jsutil.FromString(strings.Repeat("-", place.mismatchOffset)),
		jsutil.FromString("^"),
	)

	return &jsutil.Error{
		Name:           "SyntaxMatchError",
		RawMessage:     jsutil.FromString(message),
		Message:        full,
		Offset:         place.start.Offset,
		Line:           place.start.Line,
		Column:         place.start.Column,
		HasMismatch:    true,
		MismatchOffset: place.mismatchOffset,
		MismatchLength: place.mismatchLength,
		Source:         place.css,
	}
}
