package lexer

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/syntaxdef"
)

// shouldPutToTrace reports whether a syntax is one the trace names: the trace lists the types,
// properties and keywords a node was reached through, not the groups between them.
func shouldPutToTrace(syntax *syntaxdef.Node) bool {
	if syntax == nil {
		return false
	}

	return syntax.Type == "Type" || syntax.Type == "Property" || syntax.Type == "Keyword"
}

// GetTrace is the syntaxes a node was matched through, outermost first, or nothing when the node
// is not part of the match.
func (r *MatchResult) GetTrace(node *ast.Node) ([]*syntaxdef.Node, bool) {
	if r.Matched == nil {
		return nil, false
	}

	var result []*syntaxdef.Node
	found := false

	var hasMatch func(*MatchTree) bool
	hasMatch = func(matchNode *MatchTree) bool {
		if !matchNode.IsLeaf {
			for _, child := range matchNode.Match {
				if hasMatch(child) {
					if shouldPutToTrace(matchNode.Syntax) {
						result = append([]*syntaxdef.Node{matchNode.Syntax}, result...)
					}

					return true
				}
			}

			return false
		}

		if matchNode.Node == node {
			found = true
			if shouldPutToTrace(matchNode.Syntax) {
				result = []*syntaxdef.Node{matchNode.Syntax}
			} else {
				result = []*syntaxdef.Node{}
			}

			return true
		}

		return false
	}

	hasMatch(r.Matched)

	return result, found
}

// IsType reports whether a node was matched through a named type.
func (r *MatchResult) IsType(node *ast.Node, name string) bool {
	return r.testNode(node, func(syntax *syntaxdef.Node) bool {
		return syntax.Type == "Type" && syntax.Str("name").String() == name
	})
}

// IsProperty reports whether a node was matched through a named property.
func (r *MatchResult) IsProperty(node *ast.Node, name string) bool {
	return r.testNode(node, func(syntax *syntaxdef.Node) bool {
		return syntax.Type == "Property" && syntax.Str("name").String() == name
	})
}

// IsKeyword reports whether a node matched a keyword.
func (r *MatchResult) IsKeyword(node *ast.Node) bool {
	return r.testNode(node, func(syntax *syntaxdef.Node) bool {
		return syntax.Type == "Keyword"
	})
}

func (r *MatchResult) testNode(node *ast.Node, fn func(*syntaxdef.Node) bool) bool {
	trace, found := r.GetTrace(node)
	if !found {
		return false
	}

	for _, syntax := range trace {
		if fn(syntax) {
			return true
		}
	}

	return false
}
