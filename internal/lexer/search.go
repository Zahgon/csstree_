package lexer

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/walker"
)

// Fragment is a run of nodes of a value that together matched one named syntax.
type Fragment struct {
	Parent *ast.List
	Nodes  []*ast.Node
}

// getFirstMatchNode is the node the first leaf of a match tree came from.
func getFirstMatchNode(matchNode *MatchTree) *ast.Node {
	if matchNode.IsLeaf {
		return matchNode.Node
	}
	if len(matchNode.Match) == 0 {
		return nil
	}
	return getFirstMatchNode(matchNode.Match[0])
}

// getLastMatchNode is the node the last leaf of a match tree came from.
func getLastMatchNode(matchNode *MatchTree) *ast.Node {
	if matchNode.IsLeaf {
		return matchNode.Node
	}
	if len(matchNode.Match) == 0 {
		return nil
	}
	return getLastMatchNode(matchNode.Match[len(matchNode.Match)-1])
}

// MatchFragments collects the runs of nodes that matched a syntax of the given type and name.
func MatchFragments(value *ast.Node, result *MatchResult, syntaxType, name string) []Fragment {
	fragments := []Fragment{}

	if result == nil || result.Matched == nil || value == nil {
		return fragments
	}

	var findFragments func(*MatchTree)
	findFragments = func(matchNode *MatchTree) {
		if matchNode.Syntax != nil &&
			matchNode.Syntax.Type == syntaxType &&
			matchNode.Syntax.Str("name").String() == name {
			start := getFirstMatchNode(matchNode)
			end := getLastMatchNode(matchNode)

			walker.Walk(value, walker.Options{
				Enter: func(node *ast.Node, item *ast.Item, list *ast.List, _ *walker.Context) {
					if node != start || item == nil {
						return
					}

					nodes := []*ast.Node{}
					current := item

					for current != nil {
						nodes = append(nodes, current.Data)

						if current.Data == end {
							break
						}

						current = current.Next
					}

					fragments = append(fragments, Fragment{Parent: list, Nodes: nodes})
				},
			})
		}

		if !matchNode.IsLeaf {
			for _, child := range matchNode.Match {
				findFragments(child)
			}
		}
	}

	findFragments(result.Matched)

	return fragments
}

// FindValueFragments matches a value against a property and collects the runs of nodes that
// matched a named syntax.
func (l *Lexer) FindValueFragments(propertyName []uint16, value *ast.Node, syntaxType, name string) []Fragment {
	result := l.MatchProperty(propertyName, NodeValue(value))
	return MatchFragments(value, result, syntaxType, name)
}
