package generator

import (
	"strconv"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/tokenizer"
)

const (
	space            = 0x0020 // U+0020 SPACE
	quotationMark    = 0x0022 // U+0022 QUOTATION MARK (")
	apostrophe       = 0x0027 // U+0027 APOSTROPHE (')
	leftParenthesis  = 0x0028 // U+0028 LEFT PARENTHESIS (()
	rightParenthesis = 0x0029 // U+0029 RIGHT PARENTHESIS ())
)

// nodeGenerator writes one node of a known type.
type nodeGenerator func(g *generator, node *ast.Node) *jsutil.Error

// nodeGenerators is the generate half of every node type of the default syntax, the set the
// original's index-generate module exports. It is filled on start-up because its entries reach
// back into it through the node handler.
var nodeGenerators = map[string]nodeGenerator{}

func init() {
	nodeGenerators = map[string]nodeGenerator{
		"AnPlusB":               generateAnPlusB,
		"Atrule":                generateAtrule,
		"AtrulePrelude":         generateAtrulePrelude,
		"AttributeSelector":     generateAttributeSelector,
		"Block":                 generateBlock,
		"Brackets":              generateBrackets,
		"CDC":                   generateCDC,
		"CDO":                   generateCDO,
		"ClassSelector":         generateClassSelector,
		"Combinator":            generateCombinator,
		"Comment":               generateComment,
		"Condition":             generateCondition,
		"Declaration":           generateDeclaration,
		"DeclarationList":       generateDeclarationList,
		"Dimension":             generateDimension,
		"Feature":               generateFeature,
		"FeatureFunction":       generateFeatureFunction,
		"FeatureRange":          generateFeatureRange,
		"Function":              generateFunction,
		"GeneralEnclosed":       generateGeneralEnclosed,
		"Hash":                  generateHash,
		"Identifier":            generateIdentifier,
		"IdSelector":            generateIDSelector,
		"Layer":                 generateLayer,
		"LayerList":             generateLayerList,
		"MediaQuery":            generateMediaQuery,
		"MediaQueryList":        generateMediaQueryList,
		"NestingSelector":       generateNestingSelector,
		"Nth":                   generateNth,
		"Number":                generateNumber,
		"Operator":              generateOperator,
		"Parentheses":           generateParentheses,
		"Percentage":            generatePercentage,
		"PseudoClassSelector":   generatePseudoClassSelector,
		"PseudoElementSelector": generatePseudoElementSelector,
		"Ratio":                 generateRatio,
		"Raw":                   generateRaw,
		"Rule":                  generateRule,
		"Scope":                 generateScope,
		"Selector":              generateSelector,
		"SelectorList":          generateSelectorList,
		"String":                generateString,
		"StyleSheet":            generateStyleSheet,
		"SupportsDeclaration":   generateSupportsDeclaration,
		"TypeSelector":          generateTypeSelector,
		"UnicodeRange":          generateUnicodeRange,
		"Url":                   generateURL,
		"Value":                 generateValue,
		"WhiteSpace":            generateWhiteSpace,
	}
}

func generateAnPlusB(g *generator, node *ast.Node) *jsutil.Error {
	if fieldIsTruthy(node, "a") {
		var a jsutil.Str
		switch text, isString := strField(node, "a"); {
		case isString && (text.EqualString("+1") || text.EqualString("1")):
			a = jsutil.FromString("n")
		case isString && text.EqualString("-1"):
			a = jsutil.FromString("-n")
		default:
			a = jsutil.Concat(stringField(node, "a"), jsutil.FromString("n"))
		}

		if fieldIsTruthy(node, "b") {
			b := stringField(node, "b")
			// Only a string answers an index, so a value of another type always takes the sign.
			if text, isString := strField(node, "b"); !isString ||
				(text.CharCodeAt(0) != '-' && text.CharCodeAt(0) != '+') {
				b = jsutil.Concat(jsutil.FromString("+"), b)
			}
			g.tokenize(jsutil.Concat(a, b))
		} else {
			g.tokenize(a)
		}
	} else {
		g.tokenizeField(node, "b")
	}

	return nil
}

func generateAtrule(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.AtKeyword, jsutil.Concat(jsutil.FromString("@"), stringField(node, "name")), false)

	if !fieldIsNull(node, "prelude") {
		if err := g.child(node, "prelude"); err != nil {
			return err
		}
	}

	if fieldIsTruthy(node, "block") {
		if err := g.child(node, "block"); err != nil {
			return err
		}
	} else {
		g.token(tokenizer.Semicolon, jsutil.FromString(";"), false)
	}

	return nil
}

func generateAtrulePrelude(g *generator, node *ast.Node) *jsutil.Error {
	return g.children(node, nil)
}

func generateAttributeSelector(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Delim, jsutil.FromString("["), false)

	if err := g.child(node, "name"); err != nil {
		return err
	}

	if !fieldIsNull(node, "matcher") {
		g.tokenizeField(node, "matcher")
		if err := g.child(node, "value"); err != nil {
			return err
		}
	}

	if !fieldIsNull(node, "flags") {
		g.token(tokenizer.Ident, stringField(node, "flags"), false)
	}

	g.token(tokenizer.Delim, jsutil.FromString("]"), false)

	return nil
}

func generateBlock(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.LeftCurlyBracket, jsutil.FromString("{"), false)

	if err := g.children(node, g.semicolonAfterDeclaration); err != nil {
		return err
	}

	g.token(tokenizer.RightCurlyBracket, jsutil.FromString("}"), false)

	return nil
}

// semicolonAfterDeclaration writes the semicolon that closes a declaration before the next child.
func (g *generator) semicolonAfterDeclaration(prev *ast.Node) *jsutil.Error {
	if prev.Type == "Declaration" {
		g.token(tokenizer.Semicolon, jsutil.FromString(";"), false)
	}
	return nil
}

func generateBrackets(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Delim, jsutil.FromString("["), false)

	if err := g.children(node, nil); err != nil {
		return err
	}

	g.token(tokenizer.Delim, jsutil.FromString("]"), false)

	return nil
}

func generateCDC(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.CDC, jsutil.FromString("-->"), false)
	return nil
}

func generateCDO(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.CDO, jsutil.FromString("<!--"), false)
	return nil
}

func generateClassSelector(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Delim, jsutil.FromString("."), false)
	g.token(tokenizer.Ident, stringField(node, "name"), false)
	return nil
}

func generateCombinator(g *generator, node *ast.Node) *jsutil.Error {
	g.tokenizeField(node, "name")
	return nil
}

func generateComment(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Comment, jsutil.Concat(
		jsutil.FromString("/*"),
		stringField(node, "value"),
		jsutil.FromString("*/"),
	), false)
	return nil
}

func generateCondition(g *generator, node *ast.Node) *jsutil.Error {
	var err *jsutil.Error

	node.List().Each(func(child *ast.Node, item *ast.Item, list *ast.List) {
		if err != nil {
			return
		}
		if child != nil && child.Type == "Condition" {
			g.token(tokenizer.LeftParenthesis, jsutil.FromString("("), false)
			if err = g.node(child); err != nil {
				return
			}
			g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)
			return
		}
		err = g.node(child)
	})

	return err
}

func generateDeclaration(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Ident, stringField(node, "property"), false)
	g.token(tokenizer.Colon, jsutil.FromString(":"), false)

	if err := g.child(node, "value"); err != nil {
		return err
	}

	if fieldIsTruthy(node, "important") {
		g.token(tokenizer.Delim, jsutil.FromString("!"), false)

		important := stringField(node, "important")
		if flag, ok := node.Get("important").(bool); ok && flag {
			important = jsutil.FromString("important")
		}
		g.token(tokenizer.Ident, important, false)
	}

	return nil
}

func generateDeclarationList(g *generator, node *ast.Node) *jsutil.Error {
	return g.children(node, g.semicolonAfterDeclaration)
}

func generateDimension(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Dimension, jsAdd(fieldValue(node, "value"), fieldValue(node, "unit")), false)
	return nil
}

func generateFeature(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.LeftParenthesis, jsutil.FromString("("), false)
	g.token(tokenizer.Ident, stringField(node, "name"), false)

	if !fieldIsNull(node, "value") {
		g.token(tokenizer.Colon, jsutil.FromString(":"), false)
		if err := g.child(node, "value"); err != nil {
			return err
		}
	}

	g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)

	return nil
}

func generateFeatureFunction(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Function, jsutil.Concat(stringField(node, "feature"), jsutil.FromString("(")), false)

	if err := g.child(node, "value"); err != nil {
		return err
	}

	g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)

	return nil
}

func generateFeatureRange(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.LeftParenthesis, jsutil.FromString("("), false)

	if err := g.child(node, "left"); err != nil {
		return err
	}

	g.tokenizeField(node, "leftComparison")

	if err := g.child(node, "middle"); err != nil {
		return err
	}

	if fieldIsTruthy(node, "right") {
		g.tokenizeField(node, "rightComparison")
		if err := g.child(node, "right"); err != nil {
			return err
		}
	}

	g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)

	return nil
}

func generateFunction(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Function, jsutil.Concat(stringField(node, "name"), jsutil.FromString("(")), false)

	if err := g.children(node, nil); err != nil {
		return err
	}

	g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)

	return nil
}

func generateGeneralEnclosed(g *generator, node *ast.Node) *jsutil.Error {
	if fieldIsTruthy(node, "function") {
		g.token(tokenizer.Function, jsutil.Concat(stringField(node, "function"), jsutil.FromString("(")), false)
	} else {
		g.token(tokenizer.LeftParenthesis, jsutil.FromString("("), false)
	}

	if err := g.children(node, nil); err != nil {
		return err
	}

	g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)

	return nil
}

func generateHash(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Hash, jsutil.Concat(jsutil.FromString("#"), stringField(node, "value")), false)
	return nil
}

func generateIdentifier(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Ident, stringField(node, "name"), false)
	return nil
}

func generateIDSelector(g *generator, node *ast.Node) *jsutil.Error {
	// Using Delim instead of Hash is a hack to avoid for a whitespace between ident and
	// id-selector in safe mode (e.g. "a#id"), because IE11 doesn't allow a sequence <ident-token>
	// <hash-token> without a whitespace in values (e.g. "1px solid#000")
	g.token(tokenizer.Delim, jsutil.Concat(jsutil.FromString("#"), stringField(node, "name")), false)
	return nil
}

func generateLayer(g *generator, node *ast.Node) *jsutil.Error {
	g.tokenizeField(node, "name")
	return nil
}

func generateLayerList(g *generator, node *ast.Node) *jsutil.Error {
	return g.children(node, g.comma)
}

// comma writes the separator a comma separated list takes between its children.
func (g *generator) comma(prev *ast.Node) *jsutil.Error {
	g.token(tokenizer.Comma, jsutil.FromString(","), false)
	return nil
}

func generateMediaQuery(g *generator, node *ast.Node) *jsutil.Error {
	if fieldIsTruthy(node, "mediaType") {
		if fieldIsTruthy(node, "modifier") {
			g.token(tokenizer.Ident, stringField(node, "modifier"), false)
		}

		g.token(tokenizer.Ident, stringField(node, "mediaType"), false)

		if fieldIsTruthy(node, "condition") {
			g.token(tokenizer.Ident, jsutil.FromString("and"), false)
			if err := g.child(node, "condition"); err != nil {
				return err
			}
		}
	} else if fieldIsTruthy(node, "condition") {
		if err := g.child(node, "condition"); err != nil {
			return err
		}
	}

	return nil
}

func generateMediaQueryList(g *generator, node *ast.Node) *jsutil.Error {
	return g.children(node, g.comma)
}

func generateNestingSelector(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Delim, jsutil.FromString("&"), false)
	return nil
}

func generateNth(g *generator, node *ast.Node) *jsutil.Error {
	if err := g.child(node, "nth"); err != nil {
		return err
	}

	if !fieldIsNull(node, "selector") {
		g.token(tokenizer.Ident, jsutil.FromString("of"), false)
		if err := g.child(node, "selector"); err != nil {
			return err
		}
	}

	return nil
}

func generateNumber(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Number, stringField(node, "value"), false)
	return nil
}

func generateOperator(g *generator, node *ast.Node) *jsutil.Error {
	g.tokenizeField(node, "value")
	return nil
}

func generateParentheses(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.LeftParenthesis, jsutil.FromString("("), false)

	if err := g.children(node, nil); err != nil {
		return err
	}

	g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)

	return nil
}

func generatePercentage(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Percentage, jsutil.Concat(stringField(node, "value"), jsutil.FromString("%")), false)
	return nil
}

func generatePseudoClassSelector(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Colon, jsutil.FromString(":"), false)
	return g.pseudoBody(node)
}

func generatePseudoElementSelector(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Colon, jsutil.FromString(":"), false)
	g.token(tokenizer.Colon, jsutil.FromString(":"), false)
	return g.pseudoBody(node)
}

// pseudoBody writes the name of a pseudo selector, and the children it takes when it is written
// as a function.
func (g *generator) pseudoBody(node *ast.Node) *jsutil.Error {
	if fieldIsNull(node, "children") {
		g.token(tokenizer.Ident, stringField(node, "name"), false)
		return nil
	}

	g.token(tokenizer.Function, jsutil.Concat(stringField(node, "name"), jsutil.FromString("(")), false)

	if err := g.children(node, nil); err != nil {
		return err
	}

	g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)

	return nil
}

func generateRatio(g *generator, node *ast.Node) *jsutil.Error {
	if err := g.child(node, "left"); err != nil {
		return err
	}

	g.token(tokenizer.Delim, jsutil.FromString("/"), false)

	if fieldIsTruthy(node, "right") {
		return g.child(node, "right")
	}

	// The original writes the missing right side with `this.node(NumberToken, 1)`, which hands the
	// node handler a number rather than a node, so the type it reads is undefined.
	return unknownNodeType(undefinedStr)
}

func generateRaw(g *generator, node *ast.Node) *jsutil.Error {
	g.tokenizeField(node, "value")
	return nil
}

func generateRule(g *generator, node *ast.Node) *jsutil.Error {
	if err := g.child(node, "prelude"); err != nil {
		return err
	}
	return g.child(node, "block")
}

func generateScope(g *generator, node *ast.Node) *jsutil.Error {
	if fieldIsTruthy(node, "root") {
		g.token(tokenizer.LeftParenthesis, jsutil.FromString("("), false)
		if err := g.child(node, "root"); err != nil {
			return err
		}
		g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)
	}

	if fieldIsTruthy(node, "limit") {
		g.token(tokenizer.Ident, jsutil.FromString("to"), false)
		g.token(tokenizer.LeftParenthesis, jsutil.FromString("("), false)
		if err := g.child(node, "limit"); err != nil {
			return err
		}
		g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)
	}

	return nil
}

func generateSelector(g *generator, node *ast.Node) *jsutil.Error {
	return g.children(node, nil)
}

func generateSelectorList(g *generator, node *ast.Node) *jsutil.Error {
	return g.children(node, g.comma)
}

func generateString(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.String, encodeString(stringValue(node, "value")), false)
	return nil
}

func generateStyleSheet(g *generator, node *ast.Node) *jsutil.Error {
	return g.children(node, nil)
}

func generateSupportsDeclaration(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.LeftParenthesis, jsutil.FromString("("), false)

	if err := g.child(node, "declaration"); err != nil {
		return err
	}

	g.token(tokenizer.RightParenthesis, jsutil.FromString(")"), false)

	return nil
}

func generateTypeSelector(g *generator, node *ast.Node) *jsutil.Error {
	g.tokenizeField(node, "name")
	return nil
}

func generateUnicodeRange(g *generator, node *ast.Node) *jsutil.Error {
	g.tokenizeField(node, "value")
	return nil
}

func generateURL(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.Url, encodeURL(stringValue(node, "value")), false)
	return nil
}

func generateValue(g *generator, node *ast.Node) *jsutil.Error {
	return g.children(node, nil)
}

func generateWhiteSpace(g *generator, node *ast.Node) *jsutil.Error {
	g.token(tokenizer.WhiteSpace, stringField(node, "value"), false)
	return nil
}

// fieldValue is the raw value of a field, so that an addition can tell a number from text.
func fieldValue(node *ast.Node, key string) any {
	if !node.Has(key) {
		return undefinedField{}
	}
	return node.Get(key)
}

// undefinedField stands for a field that is not there, which reads as undefined.
type undefinedField struct{}

// stringValue is a field that holds a JavaScript string. A value of another kind has no length to
// walk over, so the encoders see nothing, exactly as the original's loop does.
func stringValue(node *ast.Node, key string) jsutil.Str {
	value, _ := strField(node, key)
	return value
}

// encodeString writes a string the way § 2.1 of CSSOM serializes one, between quotation marks.
// https://drafts.csswg.org/cssom/#serialize-a-string
func encodeString(str jsutil.Str) jsutil.Str {
	encoded := jsutil.Str{}
	wsBeforeHexIsNeeded := false

	for i := 0; i < len(str); i++ {
		code := str[i]

		// If the character is NULL (U+0000), then the REPLACEMENT CHARACTER (U+FFFD).
		if code == 0x0000 {
			encoded = append(encoded, 0xFFFD)
			continue
		}

		// If the character is in the range [\1-\1f] (U+0001 to U+001F) or is U+007F, the character
		// escaped as code point.
		// Note: Do not compare with 0x0001 since 0x0000 is precessed before
		if code <= 0x001f || code == 0x007F {
			encoded = append(encoded, '\\')
			encoded = append(encoded, jsutil.FromString(strconv.FormatUint(uint64(code), 16))...)
			wsBeforeHexIsNeeded = true
			continue
		}

		// If the character is '"' (U+0022) or "\" (U+005C), the escaped character.
		if code == quotationMark || code == reverseSolidus {
			encoded = append(encoded, '\\', code)
			wsBeforeHexIsNeeded = false
		} else {
			if wsBeforeHexIsNeeded && (tokenizer.IsHexDigit(code) || tokenizer.IsWhiteSpace(code)) {
				encoded = append(encoded, ' ')
			}

			// Otherwise, the character itself.
			encoded = append(encoded, code)
			wsBeforeHexIsNeeded = false
		}
	}

	quote := jsutil.FromString("\"")

	return jsutil.Concat(quote, encoded, quote)
}

// encodeURL writes the text of a url token, escaping what would end it early.
func encodeURL(str jsutil.Str) jsutil.Str {
	encoded := jsutil.Str{}
	wsBeforeHexIsNeeded := false

	for i := 0; i < len(str); i++ {
		code := str[i]

		// If the character is NULL (U+0000), then the REPLACEMENT CHARACTER (U+FFFD).
		if code == 0x0000 {
			encoded = append(encoded, 0xFFFD)
			continue
		}

		// If the character is in the range [\1-\1f] (U+0001 to U+001F) or is U+007F, the character
		// escaped as code point.
		// Note: Do not compare with 0x0001 since 0x0000 is precessed before
		if code <= 0x001f || code == 0x007F {
			encoded = append(encoded, '\\')
			encoded = append(encoded, jsutil.FromString(strconv.FormatUint(uint64(code), 16))...)
			wsBeforeHexIsNeeded = true
			continue
		}

		if code == space ||
			code == reverseSolidus ||
			code == quotationMark ||
			code == apostrophe ||
			code == leftParenthesis ||
			code == rightParenthesis {
			encoded = append(encoded, '\\', code)
			wsBeforeHexIsNeeded = false
		} else {
			if wsBeforeHexIsNeeded && tokenizer.IsHexDigit(code) {
				encoded = append(encoded, ' ')
			}

			encoded = append(encoded, code)
			wsBeforeHexIsNeeded = false
		}
	}

	return jsutil.Concat(jsutil.FromString("url("), encoded, jsutil.FromString(")"))
}
