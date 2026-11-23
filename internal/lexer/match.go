package lexer

import (
	"math"
	"strings"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/generator"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/syntaxdef"
	"github.com/csstree/csstree/internal/tokenizer"
)

// The four kinds of entry a match stack holds.
const (
	stubEntry = iota
	tokenEntry
	openSyntaxEntry
	closeSyntaxEntry
)

const (
	exitReasonMatch          = "Match"
	exitReasonMismatch       = "Mismatch"
	exitReasonIterationLimit = "Maximum iteration number exceeded (please fill an issue on https://github.com/csstree/csstree/issues)"
)

const iterationLimit = 15000

// genericFn matches a type the lexer knows by code rather than by a definition. It answers how
// many tokens from the current one the type takes, or a value that is not a number when it does
// not match.
type genericFn func(token *generator.Token, getNextToken func(offset int) *generator.Token, opts any) float64

// matchEntry is one step of what matched: a token, or the opening or closing of a syntax.
type matchEntry struct {
	kind   int
	syntax *syntaxdef.Node
	token  *generator.Token
	prev   *matchEntry
}

type syntaxEntry struct {
	syntax *syntaxdef.Node
	opts   any
	prev   *syntaxEntry
}

type thenEntry struct {
	nextState   *matchNode
	matchStack  *matchEntry
	syntaxStack *syntaxEntry
	prev        *thenEntry
}

type elseEntry struct {
	nextState   *matchNode
	matchStack  *matchEntry
	syntaxStack *syntaxEntry
	thenStack   *thenEntry
	tokenIndex  int
	prev        *elseEntry
}

// internalResult is what one run of the matcher leaves behind.
type internalResult struct {
	tokens       []generator.Token
	reason       string
	iterations   int
	match        *matchEntry
	longestMatch int
}

// reverseList turns the match stack, which is built from its end, around.
func reverseList(list *matchEntry) *matchEntry {
	var prev *matchEntry
	item := list

	for item != nil {
		next := item.prev
		item.prev = prev
		prev = item
		item = next
	}

	return prev
}

// areStringsEqualCaseInsensitive compares two strings, lower casing only A..Z of the first, as
// the original does.
func areStringsEqualCaseInsensitive(testStr, referenceStr jsutil.Str) bool {
	if len(testStr) != len(referenceStr) {
		return false
	}

	for i := 0; i < len(testStr); i++ {
		testCode := testStr[i]

		if testCode >= 0x0041 && testCode <= 0x005A {
			testCode = testCode | 32
		}

		if testCode != referenceStr[i] {
			return false
		}
	}

	return true
}

func isContextEdgeDelim(token *generator.Token) bool {
	if token.Type != tokenizer.Delim {
		return false
	}

	// Fix matching for unicode-range: U+30??, U+FF00-FF9F
	return !token.Value.EqualString("?")
}

func isCommaContextStart(token *generator.Token) bool {
	if token == nil {
		return true
	}

	switch token.Type {
	case tokenizer.Comma, tokenizer.Function, tokenizer.LeftParenthesis,
		tokenizer.LeftSquareBracket, tokenizer.LeftCurlyBracket:
		return true
	}

	return isContextEdgeDelim(token)
}

func isCommaContextEnd(token *generator.Token) bool {
	if token == nil {
		return true
	}

	switch token.Type {
	case tokenizer.RightParenthesis, tokenizer.RightSquareBracket, tokenizer.RightCurlyBracket:
		return true
	}

	return token.Type == tokenizer.Delim && token.Value.EqualString("/")
}

// dropHack takes the \0 and \9 hacks off the end of a keyword, as the original does before it
// compares one.
func dropHack(name jsutil.Str) jsutil.Str {
	text := name.String()
	if !strings.Contains(text, "\\") {
		return name
	}

	for i := 0; i+1 < len(text); i++ {
		if text[i] == '\\' && (text[i+1] == '0' || text[i+1] == '9') {
			return jsutil.FromString(text[:i])
		}
	}

	return name
}

// dictEntry is what the matcher reaches for when a graph names a type or a property.
type dictEntry struct {
	match    *matchNode
	matchRef *matchNode
}

// dictionary is the two maps the matcher looks a named syntax up in.
type dictionary struct {
	types      map[string]*dictEntry
	properties map[string]*dictEntry
	hasTypes   bool
	hasProps   bool
}

// internalMatch walks the tokens through the graph and reports what matched.
func internalMatch(tokens []generator.Token, start *matchNode, syntaxes *dictionary) (*internalResult, *jsutil.Error) {
	var syntaxStack *syntaxEntry
	var thenStack *thenEntry
	var elseStack *elseEntry

	// nil – stashing allowed, nothing stashed
	// stashDisabled – stashing disabled, nothing stashed
	// anything else – fail stashable syntaxes, some syntax stashed
	var syntaxStash *elseEntry
	stashDisabled := false

	iterationCount := 0
	exitReason := ""
	state := start

	var token *generator.Token
	tokenIndex := -1
	longestMatch := 0
	matchStack := &matchEntry{kind: stubEntry}

	moveToNextToken := func() {
		for {
			tokenIndex++
			if tokenIndex < len(tokens) {
				token = &tokens[tokenIndex]
			} else {
				token = nil
			}
			if token == nil || (token.Type != tokenizer.WhiteSpace && token.Type != tokenizer.Comment) {
				return
			}
		}
	}

	getNextToken := func(offset int) *generator.Token {
		nextIndex := tokenIndex + offset
		if nextIndex >= 0 && nextIndex < len(tokens) {
			return &tokens[nextIndex]
		}
		return nil
	}

	stateSnapshotFromSyntax := func(nextState *matchNode, prev *elseEntry) *elseEntry {
		return &elseEntry{
			nextState:   nextState,
			matchStack:  matchStack,
			syntaxStack: syntaxStack,
			thenStack:   thenStack,
			tokenIndex:  tokenIndex,
			prev:        prev,
		}
	}

	pushThenStack := func(nextState *matchNode) {
		thenStack = &thenEntry{
			nextState:   nextState,
			matchStack:  matchStack,
			syntaxStack: syntaxStack,
			prev:        thenStack,
		}
	}

	pushElseStack := func(nextState *matchNode) {
		elseStack = stateSnapshotFromSyntax(nextState, elseStack)
	}

	addTokenToMatch := func() {
		matchStack = &matchEntry{kind: tokenEntry, syntax: state.Syntax, token: token, prev: matchStack}

		moveToNextToken()
		syntaxStash = nil
		stashDisabled = false

		if tokenIndex > longestMatch {
			longestMatch = tokenIndex
		}
	}

	openSyntax := func() {
		var opts any
		if state.Syntax != nil {
			opts = state.Syntax.Get("opts")
		}
		if opts == nil && syntaxStack != nil {
			opts = syntaxStack.opts
		}

		syntaxStack = &syntaxEntry{syntax: state.Syntax, opts: opts, prev: syntaxStack}

		matchStack = &matchEntry{kind: openSyntaxEntry, syntax: state.Syntax, token: matchStack.token, prev: matchStack}
	}

	closeSyntax := func() {
		if matchStack.kind == openSyntaxEntry {
			matchStack = matchStack.prev
		} else {
			matchStack = &matchEntry{kind: closeSyntaxEntry, syntax: syntaxStack.syntax, token: matchStack.token, prev: matchStack}
		}

		syntaxStack = syntaxStack.prev
	}

	moveToNextToken()

	for exitReason == "" {
		iterationCount++
		if iterationCount >= iterationLimit {
			break
		}

		switch state.Type {
		case "Match":
			if thenStack == nil {
				// turn to a mismatch when some tokens are left unmatched
				if token != nil {
					// it does not mismatch if just one token is left and it is an IE hack
					if tokenIndex != len(tokens)-1 ||
						(!token.Value.EqualString("\\0") && !token.Value.EqualString("\\9")) {
						state = matchResultFail
						break
					}
				}

				exitReason = exitReasonMatch
				break
			}

			// go to the next syntax (the then branch)
			state = thenStack.nextState

			// check the match is not empty
			if state == matchDisallowZero {
				if thenStack.matchStack == matchStack {
					state = matchResultFail
					break
				}
				state = matchResultOK
			}

			// close a syntax if needed
			for thenStack.syntaxStack != syntaxStack {
				closeSyntax()
			}

			thenStack = thenStack.prev

		case "Mismatch":
			// when some syntax is stashed
			if syntaxStash != nil && !stashDisabled {
				// there are no else branches, or a branch reduces the match stack
				if elseStack == nil || tokenIndex > elseStack.tokenIndex {
					elseStack = syntaxStash
					syntaxStash = nil
					stashDisabled = true
				}
			} else if elseStack == nil {
				exitReason = exitReasonMismatch
				break
			}

			if elseStack == nil {
				exitReason = exitReasonMismatch
				break
			}

			// go to the next syntax (the else branch)
			state = elseStack.nextState

			// restore all the rest of the stack states
			thenStack = elseStack.thenStack
			syntaxStack = elseStack.syntaxStack
			matchStack = elseStack.matchStack
			tokenIndex = elseStack.tokenIndex
			if tokenIndex < len(tokens) {
				token = &tokens[tokenIndex]
			} else {
				token = nil
			}

			elseStack = elseStack.prev

		case "MatchGraph":
			state = state.Match

		case "If":
			// IMPORTANT: the else stack push must go first, since it stores the state of the
			// then stack before it changes.
			if state.Else != matchResultFail {
				pushElseStack(state.Else)
			}

			if state.Then != matchResultOK {
				pushThenStack(state.Then)
			}

			state = state.Match

		case "MatchOnce":
			state = &matchNode{Type: "MatchOnceBuffer", Ref: state, Index: 0, Mask: 0}

		case "MatchOnceBuffer":
			terms := state.Ref.Terms

			if state.Index == len(terms) {
				// no matches at all, or all terms were required
				if state.Mask == 0 || state.Ref.All {
					state = matchResultFail
					break
				}

				// a partial match is fine
				state = matchResultOK
				break
			}

			// all terms are matched
			if state.Mask == (1<<len(terms))-1 {
				state = matchResultOK
				break
			}

			for ; state.Index < len(terms); state.Index++ {
				matchFlag := 1 << state.Index

				if state.Mask&matchFlag == 0 {
					// IMPORTANT: the else stack push must go first.
					pushElseStack(state)
					pushThenStack(&matchNode{Type: "AddMatchOnce", Ref: state.Ref, Mask: state.Mask | matchFlag})

					next := terms[state.Index]
					state.Index++
					state = next
					break
				}
			}

		case "AddMatchOnce":
			state = &matchNode{Type: "MatchOnceBuffer", Ref: state.Ref, Index: 0, Mask: state.Mask}

		case "Enum":
			matched := false

			if token != nil {
				name := strings.ToLower(dropHack(token.Value).String())

				if next, ok := state.Map[name]; ok {
					state = next
					matched = true
				}
			}

			if !matched {
				state = matchResultFail
			}

		case "Generic":
			var opts any
			if syntaxStack != nil {
				opts = syntaxStack.opts
			}
			taken := state.Fn(token, getNextToken, opts)
			lastTokenIndex := float64(tokenIndex) + math.Floor(taken)

			if !math.IsNaN(lastTokenIndex) && lastTokenIndex > float64(tokenIndex) {
				for float64(tokenIndex) < lastTokenIndex {
					addTokenToMatch()
				}

				state = matchResultOK
			} else {
				state = matchResultFail
			}

		case "Type", "Property":
			var entry *dictEntry
			if state.Type == "Type" {
				if syntaxes.hasTypes {
					entry = syntaxes.types[state.Name]
				}
			} else if syntaxes.hasProps {
				entry = syntaxes.properties[state.Name]
			}

			if entry == nil || entry.match == nil {
				reference := "<" + state.Name + ">"
				if state.Type == "Property" {
					reference = "<'" + state.Name + "'>"
				}
				return nil, jsutil.NewError("Error", jsutil.FromString("Bad syntax reference: "+reference))
			}

			// stash a syntax for types with low priority
			if !stashDisabled && token != nil && state.Type == "Type" {
				lowPriorityMatching :=
					// https://drafts.csswg.org/css-values-4/#custom-idents
					// When parsing positionally ambiguous keywords in a property value, a
					// <custom-ident> can only claim the keyword if no other unfulfilled
					// production can claim it.
					(state.Name == "custom-ident" && token.Type == tokenizer.Ident) ||

						// https://drafts.csswg.org/css-values-4/#lengths
						// ... if a 0 could be parsed as either a <number> or a <length>, it must
						// parse as a <number>
						(state.Name == "length" && token.Value.EqualString("0"))

				if lowPriorityMatching {
					if syntaxStash == nil {
						syntaxStash = stateSnapshotFromSyntax(state, elseStack)
					}

					state = matchResultFail
					break
				}
			}

			openSyntax()
			if entry.matchRef != nil {
				state = entry.matchRef
			} else {
				state = entry.match
			}

		case "Keyword":
			matched := false

			if token != nil {
				keywordName := dropHack(token.Value)

				if areStringsEqualCaseInsensitive(keywordName, jsutil.FromString(state.Name)) {
					addTokenToMatch()
					state = matchResultOK
					matched = true
				}
			}

			if !matched {
				state = matchResultFail
			}

		case "AtKeyword", "Function":
			if token != nil && areStringsEqualCaseInsensitive(token.Value, jsutil.FromString(state.Name)) {
				addTokenToMatch()
				state = matchResultOK
				break
			}

			state = matchResultFail

		case "Token":
			if token != nil && token.Value.Equal(state.Value) {
				addTokenToMatch()
				state = matchResultOK
				break
			}

			state = matchResultFail

		case "Comma":
			if token != nil && token.Type == tokenizer.Comma {
				if isCommaContextStart(matchStack.token) {
					state = matchResultFail
				} else {
					addTokenToMatch()
					if isCommaContextEnd(token) {
						state = matchResultFail
					} else {
						state = matchResultOK
					}
				}
			} else if isCommaContextStart(matchStack.token) || isCommaContextEnd(token) {
				state = matchResultOK
			} else {
				state = matchResultFail
			}

		case "String":
			text := jsutil.Str{}
			lastTokenIndex := tokenIndex

			for ; lastTokenIndex < len(tokens) && len(text) < len(state.Value); lastTokenIndex++ {
				text = append(text, tokens[lastTokenIndex].Value...)
			}

			if areStringsEqualCaseInsensitive(text, state.Value) {
				for tokenIndex < lastTokenIndex {
					addTokenToMatch()
				}

				state = matchResultOK
			} else {
				state = matchResultFail
			}

		default:
			return nil, jsutil.NewError("Error", jsutil.FromString("Unknown node type: "+state.Type))
		}
	}

	switch exitReason {
	case "":
		exitReason = exitReasonIterationLimit
		matchStack = nil

	case exitReasonMatch:
		for syntaxStack != nil {
			closeSyntax()
		}

	default:
		matchStack = nil
	}

	return &internalResult{
		tokens:       tokens,
		reason:       exitReason,
		iterations:   iterationCount,
		match:        matchStack,
		longestMatch: longestMatch,
	}, nil
}

// MatchTree is the tree of syntaxes a value matched: a syntax and, under it, what matched it.
type MatchTree struct {
	Syntax *syntaxdef.Node
	Match  []*MatchTree

	// A leaf holds the token it matched and the node that token came from.
	IsLeaf bool
	Token  jsutil.Str
	Node   *ast.Node
}

// matchAsTree walks the tokens through the graph and builds the tree of what matched.
func matchAsTree(tokens []generator.Token, graph *matchNode, syntaxes *dictionary) (*internalResult, *MatchTree, *jsutil.Error) {
	result, err := internalMatch(tokens, graph, syntaxes)
	if err != nil {
		return nil, nil, err
	}

	if result.match == nil {
		return result, nil, nil
	}

	host := &MatchTree{Syntax: graph.Syntax}
	hostStack := []*MatchTree{host}

	// revert the list and start with the second item, since the first is a stub
	item := reverseList(result.match).prev

	for item != nil {
		switch item.kind {
		case openSyntaxEntry:
			child := &MatchTree{Syntax: item.syntax}
			host.Match = append(host.Match, child)
			host = child
			hostStack = append(hostStack, host)

		case closeSyntaxEntry:
			hostStack = hostStack[:len(hostStack)-1]
			host = hostStack[len(hostStack)-1]

		default:
			host.Match = append(host.Match, &MatchTree{
				Syntax: item.syntax,
				IsLeaf: true,
				Token:  item.token.Value,
				Node:   item.token.Node,
			})
		}

		item = item.prev
	}

	return result, hostStack[0], nil
}
