package syntaxdef

import "github.com/csstree/csstree/internal/jsutil"

// Walk visits every node of a value definition syntax tree, calling enter on the way down and
// leave on the way up. Either one may be nil, but not both.
//
// Walk panics with a *jsutil.Error when neither handler is given and when it meets a node of a
// type the original does not know, which is what the original throws for each of them; every tree
// Parse builds is walked without one.
func Walk(node *Node, enter, leave func(*Node)) {
	if enter == nil && leave == nil {
		panic(jsutil.NewError("Error", jsutil.FromString(
			"Neither `enter` nor `leave` walker handler is set or both aren't a function")))
	}

	if enter == nil {
		enter = func(*Node) {}
	}
	if leave == nil {
		leave = func(*Node) {}
	}

	walkNode(node, enter, leave)
}

func walkNode(node *Node, enter, leave func(*Node)) {
	enter(node)

	switch node.Type {
	case "Group":
		for _, term := range node.Terms("terms") {
			walkNode(term, enter, leave)
		}

	case "Multiplier", "Boolean":
		walkNode(node.Child("term"), enter, leave)

	case "Type", "Property", "Keyword", "AtKeyword", "Function", "String", "Token", "Comma":

	default:
		panic(jsutil.NewError("Error", jsutil.Concat(
			jsutil.FromString("Unknown type: "),
			jsutil.FromString(node.Type),
		)))
	}

	leave(node)
}
