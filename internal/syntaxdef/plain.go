package syntaxdef

import "github.com/csstree/csstree/internal/jsutil"

// ToPlain turns a tree into the plain value a JSON writer takes: an object whose keys are type
// and then the node's own fields in the order they were set, with every list of terms an array.
// It is the shape JSON.stringify finds the original's nodes in.
func ToPlain(node *Node) jsutil.Value {
	if node == nil {
		return jsutil.Null{}
	}
	object := jsutil.NewObject()
	object.SetString("type", jsutil.JSONString(jsutil.FromString(node.Type)))
	for i, key := range node.keys {
		object.SetString(key, valueToPlain(node.values[i]))
	}
	return object
}

func valueToPlain(value any) jsutil.Value {
	switch v := value.(type) {
	case nil:
		return jsutil.Null{}
	case jsutil.Str:
		return jsutil.JSONString(v)
	case bool:
		return jsutil.Bool(v)
	case float64:
		return jsutil.Num(v)
	case *Node:
		return ToPlain(v)
	case []*Node:
		items := jsutil.Array{}
		for _, term := range v {
			items = append(items, ToPlain(term))
		}
		return items
	case jsutil.Value:
		return v
	default:
		return jsutil.Null{}
	}
}

// FromPlain turns a plain value back into a tree. Every key of the object is kept, in its order,
// so a tree that does not match any node type is carried through unchanged and fails later where
// the original fails.
func FromPlain(value jsutil.Value) *Node {
	object, ok := value.(*jsutil.Object)
	if !ok {
		return nil
	}
	node := &Node{}
	for _, key := range object.Keys() {
		field, _ := object.Get(key)
		name := key.String()
		if name == "type" {
			if nodeType, ok := field.(jsutil.JSONString); ok {
				node.Type = jsutil.Str(nodeType).String()
			}
			continue
		}
		node.Set(name, valueFromPlain(field))
	}
	return node
}

func valueFromPlain(value jsutil.Value) any {
	switch v := value.(type) {
	case jsutil.Null:
		return nil
	case jsutil.JSONString:
		return jsutil.Str(v)
	case jsutil.Bool:
		return bool(v)
	case jsutil.Num:
		return float64(v)
	case jsutil.Int:
		return float64(v)
	case jsutil.Array:
		terms := make([]*Node, 0, len(v))
		for _, item := range v {
			terms = append(terms, FromPlain(item))
		}
		return terms
	case *jsutil.Object:
		return FromPlain(v)
	default:
		return v
	}
}
