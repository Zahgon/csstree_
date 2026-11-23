package syntaxdef

import (
	"sort"

	"github.com/csstree/csstree/internal/jsutil"
)

const (
	exclamationMark    = 33   // !
	numberSign         = 35   // #
	ampersand          = 38   // &
	apostrophe         = 39   // '
	leftParenthesis    = 40   // (
	rightParenthesis   = 41   // )
	asterisk           = 42   // *
	plusSign           = 43   // +
	comma              = 44   // ,
	hyperMinus         = 45   // -
	lessThanSign       = 60   // <
	greaterThanSign    = 62   // >
	questionMark       = 63   // ?
	commercialAt       = 64   // @
	leftSquareBracket  = 91   // [
	rightSquareBracket = 93   // ]
	leftCurlyBracket   = 123  // {
	verticalLine       = 124  // |
	rightCurlyBracket  = 125  // }
	infinity           = 8734 // ∞
)

// combinatorPrecedence orders the combinators the way the original groups them: the loosest
// binding one is taken apart first.
var combinatorPrecedence = map[string]int{
	" ":  1,
	"&&": 2,
	"||": 3,
	"|":  4,
}

// noStopCharCode is the value readImplicitGroup takes when nothing stops it but the end of the
// definition; it is the original's default of -1, which no code unit ever equals.
const noStopCharCode = -1

// jsSlice is Array.prototype.slice(start, end) over a list of terms, with a fresh list returned.
func jsSlice(terms []*Node, start, end int) []*Node {
	if start < 0 {
		start += len(terms)
	}
	if start < 0 {
		start = 0
	}
	if end > len(terms) {
		end = len(terms)
	}
	if end < start {
		end = start
	}
	out := make([]*Node, end-start)
	copy(out, terms[start:end])
	return out
}

// jsSplice is Array.prototype.splice(start, deleteCount, insert...) over a list of terms, applied
// in place so that every holder of the list sees the change, as the original's array does.
func jsSplice(terms *[]*Node, start, deleteCount int, insert ...*Node) {
	list := *terms
	if start < 0 {
		start += len(list)
	}
	if start < 0 {
		start = 0
	}
	if start > len(list) {
		start = len(list)
	}
	if deleteCount < 0 {
		deleteCount = 0
	}
	if start+deleteCount > len(list) {
		deleteCount = len(list) - start
	}

	out := make([]*Node, 0, len(list)-deleteCount+len(insert))
	out = append(out, list[:start]...)
	out = append(out, insert...)
	out = append(out, list[start+deleteCount:]...)
	*terms = out
}

func readMultiplierRange(s *scanner) (float64, float64) {
	var min jsutil.Str
	var max jsutil.Str
	hasMax := false

	s.eat(leftCurlyBracket)
	s.skipWs()

	min = s.scanNumber()
	s.skipWs()

	if s.charCode() == comma {
		s.pos++
		s.skipWs()

		if s.charCode() != rightCurlyBracket {
			max = s.scanNumber()
			hasMax = true
			s.skipWs()
		}
	} else {
		max = min
		hasMax = true
	}

	s.eat(rightCurlyBracket)

	if !hasMax {
		return jsutil.ParseFloat(min), 0
	}
	return jsutil.ParseFloat(min), jsutil.ParseFloat(max)
}

func readMultiplier(s *scanner) *Node {
	var min, max float64
	hasComma := false

	switch s.charCode() {
	case asterisk:
		s.pos++

		min, max = 0, 0

	case plusSign:
		s.pos++

		min, max = 1, 0

	case questionMark:
		s.pos++

		min, max = 0, 1

	case numberSign:
		s.pos++

		hasComma = true

		if s.charCode() == leftCurlyBracket {
			min, max = readMultiplierRange(s)
		} else if s.charCode() == questionMark {
			// https://www.w3.org/TR/css-values-4/#component-multipliers
			// > the # and ? multipliers may be stacked as #?
			// In this case just treat "#?" as a single multiplier
			// { min: 0, max: 0, comma: true }
			s.pos++
			min, max = 0, 0
		} else {
			min, max = 1, 0
		}

	case leftCurlyBracket:
		min, max = readMultiplierRange(s)

	default:
		return nil
	}

	return New("Multiplier").
		Set("comma", hasComma).
		Set("min", min).
		Set("max", max).
		Set("term", nil)
}

func maybeMultiplied(s *scanner, node *Node) *Node {
	multiplier := readMultiplier(s)

	if multiplier != nil {
		multiplier.Set("term", node)

		// https://www.w3.org/TR/css-values-4/#component-multipliers
		// > The + and # multipliers may be stacked as +#;
		// Represent "+#" as nested multipliers:
		// { ...<multiplier #>,
		//   term: {
		//     ...<multipler +>,
		//     term: node
		//   }
		// }
		if s.charCode() == numberSign && s.charCodeAt(s.pos-1) == plusSign {
			return maybeMultiplied(s, multiplier)
		}

		// https://www.w3.org/TR/css-values-4/#component-multipliers
		// > the # and ? multipliers, {A} and ? multipliers, and {A,B} and ? multipliers
		// > may be stacked as #?, {A}?, and {A,B}?, respectively
		// Represent "{}?" as nested multipliers as well as "+#".
		// The "#?" case is already handled above, in maybeMultiplied()
		if s.charCode() == questionMark && s.charCodeAt(s.pos-1) == rightCurlyBracket {
			return maybeMultiplied(s, multiplier)
		}

		return multiplier
	}

	return node
}

func maybeToken(s *scanner) *Node {
	ch := s.peek()

	if len(ch) == 0 {
		return nil
	}

	return maybeMultiplied(s, New("Token").Set("value", ch))
}

func readProperty(s *scanner) *Node {
	var name jsutil.Str

	s.eat(lessThanSign)
	s.eat(apostrophe)

	name = s.scanWord()

	s.eat(apostrophe)
	s.eat(greaterThanSign)

	return maybeMultiplied(s, New("Property").Set("name", name))
}

// https://drafts.csswg.org/css-values-3/#numeric-ranges
// 4.1. Range Restrictions and Range Definition Notation
//
// Range restrictions can be annotated in the numeric type notation using CSS bracketed
// range notation—[min,max]—within the angle brackets, after the identifying keyword,
// indicating a closed range between (and including) min and max.
// For example, <integer [0, 10]> indicates an integer between 0 and 10, inclusive.
func readTypeRange(s *scanner) *Node {
	// use nothing for Infinity to make AST format JSON serializable/deserializable; a bound that
	// carries a unit stops being a number and becomes the text of the number with the unit after
	// it, exactly as the original's string concatenation leaves it
	var min any // -Infinity
	var max any // Infinity
	sign := 1.0

	s.eat(leftSquareBracket)

	if s.charCode() == hyperMinus {
		s.peek()
		sign = -1
	}

	if sign == -1 && s.charCode() == infinity {
		s.peek()
	} else {
		value := sign * jsutil.ParseFloat(s.scanNumber())
		min = value

		if s.isNameCharCode() {
			min = jsutil.Concat(jsutil.FromString(jsutil.NumberToString(value)), s.scanWord())
		}
	}

	s.skipWs()
	s.eat(comma)
	s.skipWs()

	if s.charCode() == infinity {
		s.peek()
	} else {
		sign = 1

		if s.charCode() == hyperMinus {
			s.peek()
			sign = -1
		}

		value := sign * jsutil.ParseFloat(s.scanNumber())
		max = value

		if s.isNameCharCode() {
			max = jsutil.Concat(jsutil.FromString(jsutil.NumberToString(value)), s.scanWord())
		}
	}

	s.eat(rightSquareBracket)

	return New("Range").
		Set("min", min).
		Set("max", max)
}

func readType(s *scanner) *Node {
	var name jsutil.Str
	var opts any

	s.eat(lessThanSign)
	name = s.scanWord()

	// https://drafts.csswg.org/css-values-5/#boolean
	if name.EqualString("boolean-expr") {
		s.eat(leftSquareBracket)

		implicitGroup := readImplicitGroup(s, rightSquareBracket)

		s.eat(rightSquareBracket)
		s.eat(greaterThanSign)

		terms := implicitGroup.Terms("terms")
		term := implicitGroup
		if len(terms) == 1 {
			term = terms[0]
		}

		return maybeMultiplied(s, New("Boolean").Set("term", term))
	}

	if s.charCode() == leftParenthesis && s.nextCharCode() == rightParenthesis {
		s.pos += 2
		name = jsutil.Concat(name, jsutil.FromString("()"))
	}

	if s.charCodeAt(s.findWsEnd(s.pos)) == leftSquareBracket {
		s.skipWs()
		opts = readTypeRange(s)
	}

	s.eat(greaterThanSign)

	return maybeMultiplied(s, New("Type").
		Set("name", name).
		Set("opts", opts))
}

func readKeywordOrFunction(s *scanner) *Node {
	name := s.scanWord()

	if s.charCode() == leftParenthesis {
		s.pos++

		return New("Function").Set("name", name)
	}

	return maybeMultiplied(s, New("Keyword").Set("name", name))
}

func createGroup(terms []*Node, combinator string) *Node {
	return New("Group").
		Set("terms", terms).
		Set("combinator", jsutil.FromString(combinator)).
		Set("disallowEmpty", false).
		Set("explicit", false)
}

// regroupTerms takes the flat list of terms apart into nested groups, one combinator at a time
// from the loosest binding to the tightest, and reports the combinator of the list it leaves.
func regroupTerms(terms *[]*Node, combinators []string) string {
	var combinator string

	order := make([]string, len(combinators))
	copy(order, combinators)
	sort.SliceStable(order, func(a, b int) bool {
		return combinatorPrecedence[order[a]] < combinatorPrecedence[order[b]]
	})

	for len(order) > 0 {
		combinator = order[0]
		order = order[1:]

		i := 0
		subgroupStart := 0

		for ; i < len(*terms); i++ {
			term := (*terms)[i]

			if term.Type == "Combinator" {
				if term.Str("value").EqualString(combinator) {
					if subgroupStart == -1 {
						subgroupStart = i - 1
					}
					jsSplice(terms, i, 1)
					i--
				} else {
					if subgroupStart != -1 && i-subgroupStart > 1 {
						group := createGroup(jsSlice(*terms, subgroupStart, i), combinator)
						jsSplice(terms, subgroupStart, i-subgroupStart, group)
						i = subgroupStart + 1
					}
					subgroupStart = -1
				}
			}
		}

		if subgroupStart != -1 && len(order) > 0 {
			group := createGroup(jsSlice(*terms, subgroupStart, i), combinator)
			jsSplice(terms, subgroupStart, i-subgroupStart, group)
		}
	}

	return combinator
}

func readImplicitGroup(s *scanner, stopCharCode int) *Node {
	var combinators []string
	var terms []*Node
	var prevToken *Node
	prevTokenPos := s.pos
	prevTokenIsFunction := false

	addCombinator := func(value string) {
		for _, known := range combinators {
			if known == value {
				return
			}
		}
		combinators = append(combinators, value)
	}

	for int(s.charCode()) != stopCharCode {
		var token *Node
		if prevTokenIsFunction {
			token = readImplicitGroup(s, rightParenthesis)
		} else {
			token = peekToken(s)
		}

		if token == nil {
			break
		}

		if token.Type == "Spaces" {
			continue
		}

		if prevTokenIsFunction {
			inner := token.Terms("terms")

			if len(inner) == 0 {
				prevTokenIsFunction = false
				continue
			}

			if token.Str("combinator").EqualString(" ") {
				for len(inner) > 1 {
					addCombinator(" ") // a b
					first := inner[0]
					inner = inner[1:]
					token.Set("terms", inner)
					terms = append(terms, New("Combinator").Set("value", jsutil.FromString(" ")), first)
				}

				token = inner[0]
			}
		}

		if token.Type == "Combinator" {
			// check for combinator in group beginning and double combinator sequence
			if prevToken == nil || prevToken.Type == "Combinator" {
				s.pos = prevTokenPos
				s.errorText("Unexpected combinator")
			}

			addCombinator(token.Str("value").String())
		} else if prevToken != nil && prevToken.Type != "Combinator" {
			addCombinator(" ") // a b
			terms = append(terms, New("Combinator").Set("value", jsutil.FromString(" ")))
		}

		terms = append(terms, token)
		prevToken = token
		prevTokenPos = s.pos
		prevTokenIsFunction = token.Type == "Function"
	}

	// check for combinator in group ending
	if prevToken != nil && prevToken.Type == "Combinator" {
		s.pos -= prevTokenPos
		s.errorText("Unexpected combinator")
	}

	combinator := regroupTerms(&terms, combinators)
	if combinator == "" {
		combinator = " "
	}

	return New("Group").
		Set("terms", terms).
		Set("combinator", jsutil.FromString(combinator)).
		Set("disallowEmpty", false).
		Set("explicit", false)
}

func readGroup(s *scanner) *Node {
	var result *Node

	s.eat(leftSquareBracket)
	result = readImplicitGroup(s, rightSquareBracket)
	s.eat(rightSquareBracket)

	result.Set("explicit", true)

	if s.charCode() == exclamationMark {
		s.pos++
		result.Set("disallowEmpty", true)
	}

	return result
}

// peekToken reads the next term of a group, or reports nothing for a code unit that ends the
// group or that may only start a multiplier.
func peekToken(s *scanner) *Node {
	code := s.charCode()

	switch code {
	case rightSquareBracket:
		// don't eat, stop scan a group

	case leftSquareBracket:
		return maybeMultiplied(s, readGroup(s))

	case lessThanSign:
		if s.nextCharCode() == apostrophe {
			return readProperty(s)
		}
		return readType(s)

	case verticalLine:
		width := 1
		if s.nextCharCode() == verticalLine {
			width = 2
		}
		return New("Combinator").Set("value", s.substringToPos(s.pos+width))

	case ampersand:
		s.pos++
		s.eat(ampersand)

		return New("Combinator").Set("value", jsutil.FromString("&&"))

	case comma:
		s.pos++
		return New("Comma")

	case apostrophe:
		return maybeMultiplied(s, New("String").Set("value", s.scanString()))

	case charSpace, charTab, charN, charR, charF:
		return New("Spaces").Set("value", s.scanSpaces())

	case commercialAt:
		code = s.nextCharCode()

		if s.isNameCharCodeOf(code) {
			s.pos++
			return New("AtKeyword").Set("name", s.scanWord())
		}

		return maybeToken(s)

	case asterisk, plusSign, questionMark, numberSign, exclamationMark:
		// prohibited tokens (used as a multiplier start)

	case leftCurlyBracket:
		// leftCurlyBracket is allowed since mdn/data uses it w/o quoting
		// check next char isn't a number, because it's likely a disjoined multiplier
		code = s.nextCharCode()

		if code < 48 || code > 57 {
			return maybeToken(s)
		}

	default:
		if s.isNameCharCodeOf(code) {
			return readKeywordOrFunction(s)
		}

		return maybeToken(s)
	}

	return nil
}

// Parse reads a value definition syntax into a tree. A definition the scanner rejects gives the
// error the original raises, whose full message repeats the definition with the bad place marked.
func Parse(source jsutil.Str) (node *Node, err *jsutil.Error) {
	defer func() {
		if r := recover(); r != nil {
			raised, ok := r.(syntaxPanic)
			if !ok {
				panic(r)
			}
			node, err = nil, raised.err
		}
	}()

	s := newScanner(source)
	result := readImplicitGroup(s, noStopCharCode)

	if s.pos != len(source) {
		s.errorText("Unexpected input")
	}

	// reduce redundant groups with single group term
	terms := result.Terms("terms")
	if len(terms) == 1 && terms[0].Type == "Group" {
		return terms[0], nil
	}

	return result, nil
}
