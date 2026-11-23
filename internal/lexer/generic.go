package lexer

import (
	"math"
	"strings"

	"github.com/csstree/csstree/internal/generator"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/syntaxdef"
	"github.com/csstree/csstree/internal/tokenizer"
)

// The CSS mathematical functions, by what they give back.
// https://www.w3.org/TR/css-values-4/#math
var (
	calcFunctionNames       = []string{"calc(", "-moz-calc(", "-webkit-calc("}
	comparisonFunctionNames = []string{"min(", "max(", "clamp("}
	steppedFunctionNames    = []string{"round(", "mod(", "rem("}
	trigNumberFunctionNames = []string{"sin(", "cos(", "tan("}
	trigAngleFunctionNames  = []string{"asin(", "acos(", "atan(", "atan2("}
	otherNumberFunctions    = []string{"pow(", "sqrt(", "log(", "exp(", "sign("}
	expFunctionNames        = []string{"hypot("}
	signFunctionNames       = []string{"abs("}

	numberFunctionNames = concat(calcFunctionNames, comparisonFunctionNames, steppedFunctionNames,
		trigNumberFunctionNames, otherNumberFunctions, expFunctionNames, signFunctionNames)
	percentageFunctionNames = concat(calcFunctionNames, comparisonFunctionNames, steppedFunctionNames,
		expFunctionNames, signFunctionNames)
	dimensionFunctionNames = concat(calcFunctionNames, comparisonFunctionNames, steppedFunctionNames,
		trigAngleFunctionNames, expFunctionNames, signFunctionNames)
)

func concat(lists ...[]string) []string {
	out := []string{}
	for _, list := range lists {
		out = append(out, list...)
	}
	return out
}

// balanceCloseOf is the token that closes a block the token opens, or 0.
func balanceCloseOf(tokenType int) int {
	switch tokenType {
	case tokenizer.Function, tokenizer.LeftParenthesis:
		return tokenizer.RightParenthesis
	case tokenizer.LeftSquareBracket:
		return tokenizer.RightSquareBracket
	case tokenizer.LeftCurlyBracket:
		return tokenizer.RightCurlyBracket
	}
	return 0
}

func charCodeAt(str jsutil.Str, index int) uint16 {
	return str.CharCodeAt(index)
}

func eqStr(actual jsutil.Str, expected string) bool {
	return tokenizer.CmpStr(actual, 0, len(actual), expected)
}

func eqStrAny(actual jsutil.Str, expected []string) bool {
	for _, want := range expected {
		if eqStr(actual, want) {
			return true
		}
	}
	return false
}

// isPostfixIeHack reports whether the text ends in the Internet Explorer hack, as in 123\0 or
// 123px\9.
func isPostfixIeHack(str jsutil.Str, offset int) bool {
	if offset != len(str)-2 {
		return false
	}

	return charCodeAt(str, offset) == 0x005C && tokenizer.IsDigit(charCodeAt(str, offset+1))
}

// outOfRange reports whether a numeric value falls outside the bounds a type was written with.
func outOfRange(opts any, value jsutil.Str, numEnd int, hasNumEnd bool) bool {
	rangeNode, ok := opts.(*syntaxdef.Node)
	if !ok || rangeNode == nil || rangeNode.Type != "Range" {
		return false
	}

	text := value
	if hasNumEnd && numEnd != len(value) {
		text = value.Substring(0, numEnd)
	}

	num := jsutil.ParseFloat(text)
	if !isNumberText(text) || math.IsNaN(num) {
		return true
	}

	// When a bound is written as text it is a dimension, and the range is not checked, because
	// that would need a conversion the original does not do either.
	if min, isNumber := numberOpt(rangeNode, "min"); isNumber && num < min {
		return true
	}

	if max, isNumber := numberOpt(rangeNode, "max"); isNumber && num > max {
		return true
	}

	return false
}

// numberOpt reads a bound of a range when it is a number rather than text or nothing.
func numberOpt(rangeNode *syntaxdef.Node, key string) (float64, bool) {
	switch value := rangeNode.Get(key).(type) {
	case float64:
		return value, true
	case int:
		return float64(value), true
	}
	return 0, false
}

// isNumberText reports whether the whole text reads as a JavaScript number, which is what Number()
// asks for before a range is checked.
func isNumberText(text jsutil.Str) bool {
	trimmed := strings.TrimSpace(text.String())
	if trimmed == "" {
		return true // Number("") is 0
	}
	consumed := tokenizer.ConsumeNumber(jsutil.FromString(trimmed), 0)
	return consumed == len(jsutil.FromString(trimmed))
}

// consumeFunction takes a whole balanced function, however many tokens it runs to.
func consumeFunction(token *generator.Token, getNextToken func(int) *generator.Token) float64 {
	balanceCloseType := 0
	balanceStash := []int{}
	length := 0

scan:
	for token != nil {
		switch token.Type {
		case tokenizer.RightCurlyBracket, tokenizer.RightParenthesis, tokenizer.RightSquareBracket:
			if token.Type != balanceCloseType {
				break scan
			}

			balanceCloseType = balanceStash[len(balanceStash)-1]
			balanceStash = balanceStash[:len(balanceStash)-1]

			if len(balanceStash) == 0 {
				length++
				break scan
			}

		case tokenizer.Function, tokenizer.LeftParenthesis, tokenizer.LeftSquareBracket, tokenizer.LeftCurlyBracket:
			balanceStash = append(balanceStash, balanceCloseType)
			balanceCloseType = balanceCloseOf(token.Type)
		}

		length++
		token = getNextToken(length)
	}

	return float64(length)
}

// math lets a type also be written as one of the mathematical functions that gives that type.
func mathType(next genericFn, functionNames []string) genericFn {
	return func(token *generator.Token, getNextToken func(int) *generator.Token, opts any) float64 {
		if token == nil {
			return 0
		}

		if token.Type == tokenizer.Function && eqStrAny(token.Value, functionNames) {
			return consumeFunction(token, getNextToken)
		}

		return next(token, getNextToken, opts)
	}
}

// tokenTypeFn matches exactly one token of a type.
func tokenTypeFn(expected int) genericFn {
	return func(token *generator.Token, _ func(int) *generator.Token, _ any) float64 {
		if token == nil || token.Type != expected {
			return 0
		}
		return 1
	}
}

// customIdent is an identifier the author chose, which may not be a CSS-wide keyword or "default".
// https://drafts.csswg.org/css-values-4/#custom-idents
func customIdent(token *generator.Token, _ func(int) *generator.Token, _ any) float64 {
	if token == nil || token.Type != tokenizer.Ident {
		return 0
	}

	name := token.Value.ToLower()

	if eqStrAny(name, cssWideKeywords) {
		return 0
	}

	if eqStr(name, "default") {
		return 0
	}

	return 1
}

// dashedIdent is a custom identifier that begins with two hyphens.
func dashedIdent(token *generator.Token, _ func(int) *generator.Token, _ any) float64 {
	if token == nil || token.Type != tokenizer.Ident {
		return 0
	}

	if charCodeAt(token.Value, 0) != 0x002D || charCodeAt(token.Value, 1) != 0x002D {
		return 0
	}

	return 1
}

// customPropertyName is a dashed identifier other than "--" itself.
func customPropertyName(token *generator.Token, getNextToken func(int) *generator.Token, opts any) float64 {
	if dashedIdent(token, getNextToken, opts) == 0 {
		return 0
	}

	if token.Value.EqualString("--") {
		return 0
	}

	return 1
}

// hexColor is a hash of 3, 4, 6 or 8 hexadecimal digits.
// https://drafts.csswg.org/css-color-4/#hex-notation
func hexColor(token *generator.Token, _ func(int) *generator.Token, _ any) float64 {
	if token == nil || token.Type != tokenizer.Hash {
		return 0
	}

	length := len(token.Value)

	if length != 4 && length != 5 && length != 7 && length != 9 {
		return 0
	}

	for i := 1; i < length; i++ {
		if !tokenizer.IsHexDigit(charCodeAt(token.Value, i)) {
			return 0
		}
	}

	return 1
}

// idSelector is a hash whose name reads as an identifier.
func idSelector(token *generator.Token, _ func(int) *generator.Token, _ any) float64 {
	if token == nil || token.Type != tokenizer.Hash {
		return 0
	}

	if !tokenizer.IsIdentifierStart(charCodeAt(token.Value, 1), charCodeAt(token.Value, 2), charCodeAt(token.Value, 3)) {
		return 0
	}

	return 1
}

// declarationValue is everything a declaration may hold as its value.
// https://drafts.csswg.org/css-syntax/#any-value
func declarationValue(token *generator.Token, getNextToken func(int) *generator.Token, _ any) float64 {
	if token == nil {
		return 0
	}

	balanceCloseType := 0
	balanceStash := []int{}
	length := 0

scan:
	for token != nil {
		switch token.Type {
		case tokenizer.BadString, tokenizer.BadUrl:
			break scan

		case tokenizer.RightCurlyBracket, tokenizer.RightParenthesis, tokenizer.RightSquareBracket:
			if token.Type != balanceCloseType {
				break scan
			}

			balanceCloseType = balanceStash[len(balanceStash)-1]
			balanceStash = balanceStash[:len(balanceStash)-1]

		case tokenizer.Semicolon:
			if balanceCloseType == 0 {
				break scan
			}

		case tokenizer.Delim:
			if balanceCloseType == 0 && token.Value.EqualString("!") {
				break scan
			}

		case tokenizer.Function, tokenizer.LeftParenthesis, tokenizer.LeftSquareBracket, tokenizer.LeftCurlyBracket:
			balanceStash = append(balanceStash, balanceCloseType)
			balanceCloseType = balanceCloseOf(token.Type)
		}

		length++
		token = getNextToken(length)
	}

	return float64(length)
}

// anyValue is everything valid CSS may be in any context.
func anyValue(token *generator.Token, getNextToken func(int) *generator.Token, _ any) float64 {
	if token == nil {
		return 0
	}

	balanceCloseType := 0
	balanceStash := []int{}
	length := 0

scan:
	for token != nil {
		switch token.Type {
		case tokenizer.BadString, tokenizer.BadUrl:
			break scan

		case tokenizer.RightCurlyBracket, tokenizer.RightParenthesis, tokenizer.RightSquareBracket:
			if token.Type != balanceCloseType {
				break scan
			}

			balanceCloseType = balanceStash[len(balanceStash)-1]
			balanceStash = balanceStash[:len(balanceStash)-1]

		case tokenizer.Function, tokenizer.LeftParenthesis, tokenizer.LeftSquareBracket, tokenizer.LeftCurlyBracket:
			balanceStash = append(balanceStash, balanceCloseType)
			balanceCloseType = balanceCloseOf(token.Type)
		}

		length++
		token = getNextToken(length)
	}

	return float64(length)
}

// dimensionType is a number with a unit out of a set, or with any unit when the set is absent.
func dimensionType(units []string) genericFn {
	var set map[string]bool
	if units != nil {
		set = map[string]bool{}
		for _, unit := range units {
			set[unit] = true
		}
	}

	return func(token *generator.Token, _ func(int) *generator.Token, opts any) float64 {
		if token == nil || token.Type != tokenizer.Dimension {
			return 0
		}

		numberEnd := tokenizer.ConsumeNumber(token.Value, 0)

		if set != nil {
			// check for the Internet Explorer postfix hack, as in 123px\0 or 123px\9
			reverseSolidusOffset := token.Value.IndexOfFrom(jsutil.Str{0x005C}, numberEnd)
			var unit jsutil.Str
			if reverseSolidusOffset == -1 || !isPostfixIeHack(token.Value, reverseSolidusOffset) {
				unit = token.Value.Slice(numberEnd)
			} else {
				unit = token.Value.Substring(numberEnd, reverseSolidusOffset)
			}

			if !set[unit.ToLower().String()] {
				return 0
			}
		}

		if outOfRange(opts, token.Value, numberEnd, true) {
			return 0
		}

		return 1
	}
}

// percentageType is a number followed by a per cent sign.
// https://drafts.csswg.org/css-values-4/#percentages
func percentageType(token *generator.Token, _ func(int) *generator.Token, opts any) float64 {
	if token == nil || token.Type != tokenizer.Percentage {
		return 0
	}

	if outOfRange(opts, token.Value, len(token.Value)-1, true) {
		return 0
	}

	return 1
}

// zeroType matches a literal zero, and otherwise hands the token on.
// https://drafts.csswg.org/css-values-4/#numbers
func zeroType(next genericFn) genericFn {
	if next == nil {
		next = func(*generator.Token, func(int) *generator.Token, any) float64 { return 0 }
	}

	return func(token *generator.Token, getNextToken func(int) *generator.Token, opts any) float64 {
		if token != nil && token.Type == tokenizer.Number {
			if jsutil.ParseFloat(token.Value) == 0 && isNumberText(token.Value) {
				return 1
			}
		}

		return next(token, getNextToken, opts)
	}
}

// numberType is a real number.
// https://drafts.csswg.org/css-values-4/#numbers
func numberType(token *generator.Token, _ func(int) *generator.Token, opts any) float64 {
	if token == nil {
		return 0
	}

	numberEnd := tokenizer.ConsumeNumber(token.Value, 0)
	isNumber := numberEnd == len(token.Value)
	if !isNumber && !isPostfixIeHack(token.Value, numberEnd) {
		return 0
	}

	if outOfRange(opts, token.Value, numberEnd, true) {
		return 0
	}

	return 1
}

// integerType is a whole number, written with an optional sign and decimal digits.
// https://drafts.csswg.org/css-values-4/#integers
func integerType(token *generator.Token, _ func(int) *generator.Token, opts any) float64 {
	if token == nil || token.Type != tokenizer.Number {
		return 0
	}

	i := 0
	if charCodeAt(token.Value, 0) == 0x002B || charCodeAt(token.Value, 0) == 0x002D {
		i = 1
	}

	for ; i < len(token.Value); i++ {
		if !tokenizer.IsDigit(charCodeAt(token.Value, i)) {
			return 0
		}
	}

	if outOfRange(opts, token.Value, i, true) {
		return 0
	}

	return 1
}

// unitGroups are the groups of units a dimension type can be written with.
var unitGroups = []string{"length", "angle", "time", "frequency", "resolution", "flex", "decibel", "semitones"}

// attrUnitType matches any identifier that names a unit, and the per cent sign.
func attrUnitType(units map[string][]string) genericFn {
	unitSet := map[string]bool{}

	for _, group := range unitGroups {
		for _, unit := range units[group] {
			unitSet[strings.ToLower(unit)] = true
		}
	}

	return func(token *generator.Token, _ func(int) *generator.Token, _ any) float64 {
		if token == nil {
			return 0
		}

		if token.Type == tokenizer.Delim && token.Value.EqualString("%") {
			return 1
		}

		if token.Type == tokenizer.Ident && unitSet[token.Value.ToLower().String()] {
			return 1
		}

		return 0
	}
}

// tokenTypeNames pairs every token production name with the token type it names.
var tokenTypeNames = map[string]int{
	"ident-token": tokenizer.Ident, "function-token": tokenizer.Function,
	"at-keyword-token": tokenizer.AtKeyword, "hash-token": tokenizer.Hash,
	"string-token": tokenizer.String, "bad-string-token": tokenizer.BadString,
	"url-token": tokenizer.Url, "bad-url-token": tokenizer.BadUrl,
	"delim-token": tokenizer.Delim, "number-token": tokenizer.Number,
	"percentage-token": tokenizer.Percentage, "dimension-token": tokenizer.Dimension,
	"whitespace-token": tokenizer.WhiteSpace, "CDO-token": tokenizer.CDO,
	"CDC-token": tokenizer.CDC, "colon-token": tokenizer.Colon,
	"semicolon-token": tokenizer.Semicolon, "comma-token": tokenizer.Comma,
	"[-token": tokenizer.LeftSquareBracket, "]-token": tokenizer.RightSquareBracket,
	"(-token": tokenizer.LeftParenthesis, ")-token": tokenizer.RightParenthesis,
	"{-token": tokenizer.LeftCurlyBracket, "}-token": tokenizer.RightCurlyBracket,
}

// createGenericTypes builds every type the lexer matches with code of its own, for a set of units.
func createGenericTypes(units map[string][]string) map[string]genericFn {
	types := map[string]genericFn{}

	for name, tokenType := range tokenTypeNames {
		types[name] = tokenTypeFn(tokenType)
	}

	// token type aliases
	types["string"] = tokenTypeFn(tokenizer.String)
	types["ident"] = tokenTypeFn(tokenizer.Ident)

	types["percentage"] = mathType(percentageType, percentageFunctionNames)

	types["zero"] = zeroType(nil)
	types["number"] = mathType(numberType, numberFunctionNames)
	types["integer"] = mathType(integerType, numberFunctionNames)

	types["custom-ident"] = customIdent
	types["dashed-ident"] = dashedIdent
	types["custom-property-name"] = customPropertyName
	types["hex-color"] = hexColor
	types["id-selector"] = idSelector
	types["an-plus-b"] = anPlusB
	types["urange"] = urange
	types["declaration-value"] = declarationValue
	types["any-value"] = anyValue

	types["dimension"] = mathType(dimensionType(nil), dimensionFunctionNames)
	types["angle"] = mathType(dimensionType(units["angle"]), dimensionFunctionNames)
	types["decibel"] = mathType(dimensionType(units["decibel"]), dimensionFunctionNames)
	types["frequency"] = mathType(dimensionType(units["frequency"]), dimensionFunctionNames)
	types["flex"] = mathType(dimensionType(units["flex"]), dimensionFunctionNames)
	types["length"] = mathType(zeroType(dimensionType(units["length"])), dimensionFunctionNames)
	types["resolution"] = mathType(dimensionType(units["resolution"]), dimensionFunctionNames)
	types["semitones"] = mathType(dimensionType(units["semitones"]), dimensionFunctionNames)
	types["time"] = mathType(dimensionType(units["time"]), dimensionFunctionNames)

	types["attr-unit"] = attrUnitType(units)

	return types
}

// cssWideKeywords are the keywords every property accepts.
// https://drafts.csswg.org/css-cascade-5/
var cssWideKeywords = []string{"initial", "inherit", "unset", "revert", "revert-layer"}

// defaultUnits are the units each dimension type is written with.
var defaultUnits = map[string][]string{
	// absolute length units https://www.w3.org/TR/css-values-3/#lengths
	// font-relative, viewport-percentage and container relative lengths
	"length": {
		"cm", "mm", "q", "in", "pt", "pc", "px",
		"em", "rem", "ex", "rex", "cap", "rcap", "ch", "rch", "ic", "ric", "lh", "rlh",
		"vw", "svw", "lvw", "dvw", "vh", "svh", "lvh", "dvh", "vi", "svi", "lvi", "dvi",
		"vb", "svb", "lvb", "dvb", "vmin", "svmin", "lvmin", "dvmin", "vmax", "svmax", "lvmax", "dvmax",
		"cqw", "cqh", "cqi", "cqb", "cqmin", "cqmax",
	},
	"angle":      {"deg", "grad", "rad", "turn"}, // https://www.w3.org/TR/css-values-3/#angles
	"time":       {"s", "ms"},                    // https://www.w3.org/TR/css-values-3/#time
	"frequency":  {"hz", "khz"},                  // https://www.w3.org/TR/css-values-3/#frequency
	"resolution": {"dpi", "dpcm", "dppx", "x"},   // https://www.w3.org/TR/css-values-3/#resolution
	"flex":       {"fr"},                         // https://drafts.csswg.org/css-grid/#fr-unit
	"decibel":    {"db"},                         // https://www.w3.org/TR/css3-speech/#mixing-props-voice-volume
	"semitones":  {"st"},                         // https://www.w3.org/TR/css3-speech/#voice-props-voice-pitch
}
