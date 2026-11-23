package syntaxdef

import "github.com/csstree/csstree/internal/jsutil"

// text is the string a field of a node reads as when it is written into a definition: a string
// field as it is, a number the way JavaScript prints a number.
func text(value any) jsutil.Str {
	switch v := value.(type) {
	case jsutil.Str:
		return v
	case float64:
		return jsutil.FromString(jsutil.NumberToString(v))
	case bool:
		if v {
			return jsutil.FromString("true")
		}
		return jsutil.FromString("false")
	case nil:
		return jsutil.FromString("null")
	default:
		return jsutil.FromString("undefined")
	}
}

func generateMultiplier(multiplier *Node) jsutil.Str {
	min := multiplier.Num("min")
	max := multiplier.Num("max")
	comma := multiplier.Bool("comma")

	if min == 0 && max == 0 {
		if comma {
			return jsutil.FromString("#?")
		}
		return jsutil.FromString("*")
	}

	if min == 0 && max == 1 {
		return jsutil.FromString("?")
	}

	if min == 1 && max == 0 {
		if comma {
			return jsutil.FromString("#")
		}
		return jsutil.FromString("+")
	}

	if min == 1 && max == 1 {
		return jsutil.Str{}
	}

	var result jsutil.Str
	if comma {
		result = jsutil.FromString("#")
	}

	if min == max {
		return jsutil.Concat(result,
			jsutil.FromString("{"), text(min), jsutil.FromString("}"))
	}

	upper := jsutil.Str{}
	if max != 0 {
		upper = text(max)
	}

	return jsutil.Concat(result,
		jsutil.FromString("{"), text(min), jsutil.FromString(","), upper, jsutil.FromString("}"))
}

// generateTypeOpts writes the bracketed range notation of a type, such as the ` [0,10]` of
// `<integer [0,10]>`. It panics for a node that is not a range, as the original throws for one.
func generateTypeOpts(node *Node) jsutil.Str {
	switch node.Type {
	case "Range":
		min := jsutil.FromString("-∞")
		if value := node.Get("min"); value != nil {
			min = text(value)
		}

		max := jsutil.FromString("∞")
		if value := node.Get("max"); value != nil {
			max = text(value)
		}

		return jsutil.Concat(
			jsutil.FromString(" ["),
			min,
			jsutil.FromString(","),
			max,
			jsutil.FromString("]"),
		)

	default:
		panic(unknownNodeType(node.Type))
	}
}

func generateSequence(node *Node, forceBraces, compact bool) jsutil.Str {
	combinator := node.Str("combinator")
	if !combinator.EqualString(" ") && !compact {
		combinator = jsutil.Concat(jsutil.FromString(" "), combinator, jsutil.FromString(" "))
	}

	var result jsutil.Str
	for i, term := range node.Terms("terms") {
		if i > 0 {
			result = jsutil.Concat(result, combinator)
		}
		result = jsutil.Concat(result, internalGenerate(term, forceBraces, compact))
	}

	if node.Bool("explicit") || forceBraces {
		opening := jsutil.FromString("[ ")
		if compact || (len(result) > 0 && result[0] == ',') {
			opening = jsutil.FromString("[")
		}

		closing := jsutil.FromString(" ]")
		if compact {
			closing = jsutil.FromString("]")
		}

		return jsutil.Concat(opening, result, closing)
	}

	return result
}

func internalGenerate(node *Node, forceBraces, compact bool) jsutil.Str {
	var result jsutil.Str

	switch node.Type {
	case "Group":
		result = generateSequence(node, forceBraces, compact)
		if node.Bool("disallowEmpty") {
			result = jsutil.Concat(result, jsutil.FromString("!"))
		}

	case "Multiplier":
		// A multiplier is a composition: the term it repeats is written, then the multiplier.
		return jsutil.Concat(
			internalGenerate(node.Child("term"), forceBraces, compact),
			generateMultiplier(node),
		)

	case "Boolean":
		result = jsutil.Concat(
			jsutil.FromString("<boolean-expr["),
			internalGenerate(node.Child("term"), forceBraces, compact),
			jsutil.FromString("]>"),
		)

	case "Type":
		opts := jsutil.Str{}
		if typeOpts := node.Child("opts"); typeOpts != nil {
			opts = generateTypeOpts(typeOpts)
		}
		result = jsutil.Concat(jsutil.FromString("<"), node.Str("name"), opts, jsutil.FromString(">"))

	case "Property":
		result = jsutil.Concat(jsutil.FromString("<'"), node.Str("name"), jsutil.FromString("'>"))

	case "Keyword":
		result = node.Str("name")

	case "AtKeyword":
		result = jsutil.Concat(jsutil.FromString("@"), node.Str("name"))

	case "Function":
		result = jsutil.Concat(node.Str("name"), jsutil.FromString("("))

	case "String", "Token":
		result = node.Str("value")

	case "Comma":
		result = jsutil.FromString(",")

	default:
		panic(unknownNodeType(node.Type))
	}

	return result
}

func unknownNodeType(nodeType string) *jsutil.Error {
	return jsutil.NewError("Error", jsutil.Concat(
		jsutil.FromString("Unknown node type `"),
		jsutil.FromString(nodeType),
		jsutil.FromString("`"),
	))
}

// Generate writes the value definition syntax a tree stands for. With forceBraces every group is
// written inside square brackets, and with compact the spaces around a combinator and inside a
// group are left out.
//
// A node of a type the original does not know makes Generate panic with a *jsutil.Error, which is
// what the original throws for it; every tree Parse builds is written without one.
func Generate(node *Node, forceBraces, compact bool) jsutil.Str {
	return internalGenerate(node, forceBraces, compact)
}
