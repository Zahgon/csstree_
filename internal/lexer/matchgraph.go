// Package lexer matches CSS values against the CSS grammar: it turns a value definition syntax
// into a graph of conditions, walks a value's tokens through that graph, and reports either the
// tree of syntaxes the value matched or where it stopped matching.
package lexer

import (
	"strings"

	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/syntaxdef"
)

// A node of the match graph. The graph is walked by the matcher, which reads Type to know what to
// do and follows Then and Else out of an If.
type matchNode struct {
	Type string

	// If
	Match *matchNode
	Then  *matchNode
	Else  *matchNode

	// Enum: the terms of a run of alternatives that are all keywords, by their lower cased name.
	Map map[string]*matchNode

	// MatchOnce
	Terms []*matchNode
	All   bool

	// Generic: a type the lexer matches with code of its own rather than with a definition.
	Fn genericFn

	// Type, Property, Keyword, AtKeyword, Function
	Name string

	// String, Token
	Value jsutil.Str

	// The definition node this came from, for the match tree and the error messages.
	Syntax *syntaxdef.Node

	// MatchGraph
	Source *syntaxdef.Node

	// The state of a run through a MatchOnce: which term is being tried and which have matched.
	Index int
	Mask  int
	Ref   *matchNode
}

// The three results a condition can reach. They are compared by identity, as the original
// compares them, so there is exactly one of each.
var (
	matchResultOK     = &matchNode{Type: "Match"}
	matchResultFail   = &matchNode{Type: "Mismatch"}
	matchDisallowZero = &matchNode{Type: "DisallowEmpty"}
)

const (
	leftParenthesisCode  = 40 // (
	rightParenthesisCode = 41 // )
)

// createCondition builds an If, folding away the conditions that would add nothing.
func createCondition(match, thenBranch, elseBranch *matchNode) *matchNode {
	// reduce node count
	if thenBranch == matchResultOK && elseBranch == matchResultFail {
		return match
	}

	if match == matchResultOK && thenBranch == matchResultOK && elseBranch == matchResultOK {
		return match
	}

	if match.Type == "If" && match.Else == matchResultFail && thenBranch == matchResultOK {
		thenBranch = match.Then
		match = match.Match
	}

	return &matchNode{Type: "If", Match: match, Then: thenBranch, Else: elseBranch}
}

// isFunctionType reports whether a type name names a function, which is written with a pair of
// parentheses at its end.
func isFunctionType(name string) bool {
	return len(name) > 2 && name[len(name)-2] == leftParenthesisCode && name[len(name)-1] == rightParenthesisCode
}

// isEnumCompatible reports whether a term is one a run of alternatives can be looked up by name.
func isEnumCompatible(term *syntaxdef.Node) bool {
	return term.Type == "Keyword" ||
		term.Type == "AtKeyword" ||
		term.Type == "Function" ||
		(term.Type == "Type" && isFunctionType(term.Str("name").String()))
}

// groupNode builds a definition group, for the graphs the lexer writes by itself.
func groupNode(terms []*syntaxdef.Node, combinator string, explicit bool) *syntaxdef.Node {
	node := syntaxdef.New("Group")
	node.Set("terms", terms)
	node.Set("combinator", jsutil.FromString(combinator))
	node.Set("disallowEmpty", false)
	node.Set("explicit", explicit)
	return node
}

func keywordNode(name string) *syntaxdef.Node {
	node := syntaxdef.New("Keyword")
	node.Set("name", jsutil.FromString(name))
	return node
}

// typeNode names a type in a graph the lexer writes by itself. It carries no opts, as the
// original's own nodes do not, and the absence is written out.
func typeNode(name string) *syntaxdef.Node {
	node := syntaxdef.New("Type")
	node.Set("name", jsutil.FromString(name))
	return node
}

func tokenNode(value string) *syntaxdef.Node {
	node := syntaxdef.New("Token")
	node.Set("value", jsutil.FromString(value))
	return node
}

func multiplierNode(term *syntaxdef.Node, comma bool, min, max float64) *syntaxdef.Node {
	node := syntaxdef.New("Multiplier")
	node.Set("comma", comma)
	node.Set("min", min)
	node.Set("max", max)
	node.Set("term", term)
	return node
}

// replaceTypeInGraph puts the graphs a boolean expression refers to in the place of the type
// names that stand for them, walking each node once.
func replaceTypeInGraph(node *matchNode, replacements map[string]*matchNode, visited map[*matchNode]bool) *matchNode {
	if !visited[node] {
		visited[node] = true

		switch node.Type {
		case "If":
			node.Match = replaceTypeInGraph(node.Match, replacements, visited)
			node.Then = replaceTypeInGraph(node.Then, replacements, visited)
			node.Else = replaceTypeInGraph(node.Else, replacements, visited)

		case "Type":
			if replacement, ok := replacements[node.Name]; ok {
				return replacement
			}
			return node
		}
	}

	return node
}

// buildGroupMatchGraph turns the terms of a group into the conditions its combinator means.
func buildGroupMatchGraph(combinator string, terms []*matchNode, syntaxTerms []*syntaxdef.Node, atLeastOneTermMatched bool) *matchNode {
	switch combinator {
	case " ":
		// Juxtaposing components means that all of them must occur, in the given order.
		result := matchResultOK

		for i := len(terms) - 1; i >= 0; i-- {
			result = createCondition(terms[i], result, matchResultFail)
		}

		return result

	case "|":
		// A bar separates two or more alternatives: exactly one of them must occur.
		result := matchResultFail
		var enum map[string]*matchNode

		for i := len(terms) - 1; i >= 0; i-- {
			term := terms[i]
			syntaxTerm := syntaxTerms[i]

			// reduce a run of keywords into an Enum
			if isEnumCompatible(syntaxTerm) {
				if enum == nil && i > 0 && isEnumCompatible(syntaxTerms[i-1]) {
					enum = map[string]*matchNode{}
					result = createCondition(&matchNode{Type: "Enum", Map: enum}, matchResultOK, result)
				}

				if enum != nil {
					name := term.Name
					if syntaxTerm.Type == "Type" && isFunctionType(name) {
						name = name[:len(name)-1]
					}
					key := strings.ToLower(name)
					if _, taken := enum[key]; !taken {
						enum[key] = term
						continue
					}
				}
			}

			enum = nil

			result = createCondition(term, matchResultOK, result)
		}

		return result

	case "&&":
		// A double ampersand separates two or more components, all of which must occur, in any
		// order. A group of many terms would make at least N! nodes, so it is matched by a
		// node of its own instead.
		if len(terms) > 5 {
			return &matchNode{Type: "MatchOnce", Terms: terms, All: true}
		}

		result := matchResultFail

		for i := len(terms) - 1; i >= 0; i-- {
			var thenClause *matchNode

			if len(terms) > 1 {
				thenClause = buildGroupMatchGraph(combinator, without(terms, i), without(syntaxTerms, i), false)
			} else {
				thenClause = matchResultOK
			}

			result = createCondition(terms[i], thenClause, result)
		}

		return result

	case "||":
		// A double bar separates two or more options: one or more of them must occur, in any
		// order.
		if len(terms) > 5 {
			return &matchNode{Type: "MatchOnce", Terms: terms, All: false}
		}

		result := matchResultFail
		if atLeastOneTermMatched {
			result = matchResultOK
		}

		for i := len(terms) - 1; i >= 0; i-- {
			var thenClause *matchNode

			if len(terms) > 1 {
				thenClause = buildGroupMatchGraph(combinator, without(terms, i), without(syntaxTerms, i), true)
			} else {
				thenClause = matchResultOK
			}

			result = createCondition(terms[i], thenClause, result)
		}

		return result
	}

	return nil
}

// without is the terms with the one at index left out, which is how a group of terms in any order
// is reduced one term at a time.
func without[T any](terms []T, index int) []T {
	out := make([]T, 0, len(terms)-1)
	out = append(out, terms[:index]...)
	return append(out, terms[index+1:]...)
}

// buildMultiplierMatchGraph turns a repeated term into the conditions its bounds mean.
func buildMultiplierMatchGraph(node *syntaxdef.Node) *matchNode {
	result := matchResultOK
	matchTerm := buildMatchGraphInternal(node.Child("term"))
	comma := node.Bool("comma")
	min := int(node.Num("min"))
	max := int(node.Num("max"))

	if max == 0 {
		// disable repeating of an empty match to prevent an endless loop
		matchTerm = createCondition(matchTerm, matchDisallowZero, matchResultFail)

		// an occurrence count is not limited, make a cycle, to collect more terms on each
		// following matching mismatch
		result = createCondition(matchTerm, nil, matchResultFail)

		result.Then = createCondition(matchResultOK, matchResultOK, result)

		if comma {
			result.Then.Else = createCondition(&matchNode{Type: "Comma", Syntax: node}, result, matchResultFail)
		}
	} else {
		// create a match node chain for the [min .. max] interval with optional matches
		start := min
		if start == 0 {
			start = 1
		}
		for i := start; i <= max; i++ {
			if comma && result != matchResultOK {
				result = createCondition(&matchNode{Type: "Comma", Syntax: node}, result, matchResultFail)
			}

			result = createCondition(matchTerm, createCondition(matchResultOK, matchResultOK, result), matchResultFail)
		}
	}

	if min == 0 {
		// allow zero match
		result = createCondition(matchResultOK, matchResultOK, result)
	} else {
		// create a match node chain to collect the [0 ... min - 1] required matches
		for i := 0; i < min-1; i++ {
			if comma && result != matchResultOK {
				result = createCondition(&matchNode{Type: "Comma", Syntax: node}, result, matchResultFail)
			}

			result = createCondition(matchTerm, result, matchResultFail)
		}
	}

	return result
}

// buildMatchGraphInternal turns one definition node into its part of the graph.
func buildMatchGraphInternal(node *syntaxdef.Node) *matchNode {
	switch node.Type {
	case "Group":
		syntaxTerms := node.Terms("terms")
		terms := make([]*matchNode, len(syntaxTerms))
		for i, term := range syntaxTerms {
			terms[i] = buildMatchGraphInternal(term)
		}

		result := buildGroupMatchGraph(node.Str("combinator").String(), terms, syntaxTerms, false)

		if node.Bool("disallowEmpty") {
			result = createCondition(result, matchDisallowZero, matchResultFail)
		}

		return result

	case "Multiplier":
		return buildMultiplierMatchGraph(node)

	case "Boolean":
		// https://drafts.csswg.org/css-values-5/#boolean
		term := buildMatchGraphInternal(node.Child("term"))

		// <boolean-expr[ <test> ]> =
		//   not <boolean-expr-group> |
		//   <boolean-expr-group> [ [ and <boolean-expr-group> ]* | [ or <boolean-expr-group> ]* ]
		matchNodeGraph := buildMatchGraphInternal(groupNode([]*syntaxdef.Node{
			groupNode([]*syntaxdef.Node{
				keywordNode("not"),
				typeNode("!boolean-group"),
			}, " ", false),
			groupNode([]*syntaxdef.Node{
				typeNode("!boolean-group"),
				groupNode([]*syntaxdef.Node{
					multiplierNode(groupNode([]*syntaxdef.Node{
						keywordNode("and"),
						typeNode("!boolean-group"),
					}, " ", false), false, 0, 0),
					multiplierNode(groupNode([]*syntaxdef.Node{
						keywordNode("or"),
						typeNode("!boolean-group"),
					}, " ", false), false, 0, 0),
				}, "|", false),
			}, " ", false),
		}, "|", false))

		// <boolean-expr-group> = <test> | ( <boolean-expr[ <test> ]> ) | <general-enclosed>
		booleanGroup := buildMatchGraphInternal(groupNode([]*syntaxdef.Node{
			typeNode("!term"),
			groupNode([]*syntaxdef.Node{
				tokenNode("("),
				typeNode("!self"),
				tokenNode(")"),
			}, " ", false),
			typeNode("general-enclosed"),
		}, "|", false))

		replaceTypeInGraph(booleanGroup, map[string]*matchNode{"!term": term, "!self": matchNodeGraph}, map[*matchNode]bool{})
		replaceTypeInGraph(matchNodeGraph, map[string]*matchNode{"!boolean-group": booleanGroup}, map[*matchNode]bool{})

		return matchNodeGraph

	case "Type", "Property":
		return &matchNode{Type: node.Type, Name: node.Str("name").String(), Syntax: node}

	case "Keyword":
		return &matchNode{Type: node.Type, Name: strings.ToLower(node.Str("name").String()), Syntax: node}

	case "AtKeyword":
		return &matchNode{Type: node.Type, Name: "@" + strings.ToLower(node.Str("name").String()), Syntax: node}

	case "Function":
		return &matchNode{Type: node.Type, Name: strings.ToLower(node.Str("name").String()) + "(", Syntax: node}

	case "String":
		value := node.Str("value")

		// convert a one char length String to a Token
		if len(value) == 3 {
			return &matchNode{Type: "Token", Value: value.Substring(1, 2), Syntax: node}
		}

		// otherwise use it as is
		inner := value.Substring(1, len(value)-1).String()
		return &matchNode{Type: node.Type, Value: jsutil.FromString(strings.ReplaceAll(inner, "\\'", "'")), Syntax: node}

	case "Token":
		return &matchNode{Type: node.Type, Value: node.Str("value"), Syntax: node}

	case "Comma":
		return &matchNode{Type: node.Type, Syntax: node}
	}

	panic(jsutil.NewError("Error", jsutil.FromString("Unknown node type:")))
}

// buildGenericMatchGraph wraps a type the lexer matches with code of its own.
func buildGenericMatchGraph(fn genericFn, ref *syntaxdef.Node) *matchNode {
	return &matchNode{Type: "MatchGraph", Match: &matchNode{Type: "Generic", Fn: fn}, Syntax: ref}
}

// buildMatchGraph turns a value definition syntax into the graph the matcher walks.
func buildMatchGraph(syntaxTree *syntaxdef.Node, ref *syntaxdef.Node) *matchNode {
	return &matchNode{
		Type:   "MatchGraph",
		Match:  buildMatchGraphInternal(syntaxTree),
		Syntax: ref,
		Source: syntaxTree,
	}
}

// buildMatchGraphFromText parses a definition and turns it into a graph.
func buildMatchGraphFromText(definition jsutil.Str, ref *syntaxdef.Node) (*matchNode, *jsutil.Error) {
	tree, err := syntaxdef.Parse(definition)
	if err != nil {
		return nil, err
	}
	return buildMatchGraph(tree, ref), nil
}
