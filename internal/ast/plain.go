package ast

import "github.com/csstree/csstree/internal/jsutil"

// ToPlain turns a tree into the plain value a JSON writer takes: an object whose keys are type,
// loc and then the node's own fields in the order they were set, with every list of children an
// array. It is the shape the original's toPlainObject leaves behind.
func ToPlain(node *Node) jsutil.Value {
	if node == nil {
		return jsutil.Null{}
	}
	object := jsutil.NewObject()
	object.SetString("type", jsutil.JSONString(jsutil.FromString(node.Type)))
	object.SetString("loc", locToPlain(node.Loc))
	for i, key := range node.keys {
		object.SetString(key, valueToPlain(node.values[i]))
	}
	return object
}

func locToPlain(loc *Location) jsutil.Value {
	if loc == nil {
		return jsutil.Null{}
	}
	object := jsutil.NewObject()
	object.SetString("source", jsutil.JSONString(loc.Source))
	object.SetString("start", positionToPlain(loc.Start))
	object.SetString("end", positionToPlain(loc.End))
	return object
}

func positionToPlain(p Position) jsutil.Value {
	object := jsutil.NewObject()
	object.SetString("offset", jsutil.Int(p.Offset))
	object.SetString("line", jsutil.Int(p.Line))
	object.SetString("column", jsutil.Int(p.Column))
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
	case *Node:
		return ToPlain(v)
	case *List:
		items := jsutil.Array{}
		for item := v.Head; item != nil; item = item.Next {
			items = append(items, ToPlain(item.Data))
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
		switch name {
		case "type":
			if text, ok := field.(jsutil.JSONString); ok {
				node.Type = jsutil.Str(text).String()
			}
		case "loc":
			node.Loc = locFromPlain(field)
		default:
			node.Set(name, valueFromPlain(field))
		}
	}
	return node
}

func locFromPlain(value jsutil.Value) *Location {
	object, ok := value.(*jsutil.Object)
	if !ok {
		return nil
	}
	loc := &Location{}
	if source, ok := object.Get(jsutil.FromString("source")); ok {
		if text, ok := source.(jsutil.JSONString); ok {
			loc.Source = jsutil.Str(text)
		}
	}
	loc.Start = positionFromPlain(object, "start")
	loc.End = positionFromPlain(object, "end")
	return loc
}

func positionFromPlain(object *jsutil.Object, key string) Position {
	value, ok := object.Get(jsutil.FromString(key))
	if !ok {
		return Position{}
	}
	fields, ok := value.(*jsutil.Object)
	if !ok {
		return Position{}
	}
	read := func(name string) int {
		v, ok := fields.Get(jsutil.FromString(name))
		if !ok {
			return 0
		}
		switch n := v.(type) {
		case jsutil.Int:
			return int(n)
		case jsutil.Num:
			return int(n)
		}
		return 0
	}
	return Position{Offset: read("offset"), Line: read("line"), Column: read("column")}
}

func valueFromPlain(value jsutil.Value) any {
	switch v := value.(type) {
	case jsutil.Null:
		return nil
	case jsutil.JSONString:
		return jsutil.Str(v)
	case jsutil.Bool:
		return bool(v)
	case jsutil.Array:
		list := NewList()
		for _, item := range v {
			list.AppendData(FromPlain(item))
		}
		return list
	case *jsutil.Object:
		return FromPlain(v)
	default:
		return v
	}
}

// Clone copies a tree: every node, every list and every location is new, so changing the copy
// leaves the original alone.
func Clone(node *Node) *Node {
	if node == nil {
		return nil
	}
	copied := &Node{Type: node.Type}
	if node.Loc != nil {
		loc := *node.Loc
		copied.Loc = &loc
	}
	for i, key := range node.keys {
		switch v := node.values[i].(type) {
		case *Node:
			copied.Set(key, Clone(v))
		case *List:
			list := NewList()
			for item := v.Head; item != nil; item = item.Next {
				list.AppendData(Clone(item.Data))
			}
			copied.Set(key, list)
		default:
			copied.Set(key, node.values[i])
		}
	}
	return copied
}
