// Package ast holds the CSS abstract syntax tree: the nodes the parser builds, the list of
// children a node can have, the source locations, and the declared structure of every node type.
//
// A node keeps its own fields in the order the parser set them, because that order is the order
// the tree is written out in as JSON, and it is the order the original's node objects have.
package ast

import "github.com/csstree/csstree/internal/jsutil"

// Position is one point in the source, counted in UTF-16 code units for the offset.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Location is the span of source a node came from.
type Location struct {
	Source jsutil.Str
	Start  Position
	End    Position
}

// Node is one node of the tree. Besides its type and its location it carries named fields, which
// are a string, a boolean, a child node, a list of children, or nothing.
type Node struct {
	Type string
	Loc  *Location

	keys   []string
	values []any
}

// New returns a node of the type with no fields set.
func New(nodeType string) *Node {
	return &Node{Type: nodeType}
}

// Set gives a field a value, keeping the position a field already there has, and returns the node.
// The value is a jsutil.Str, a bool, a *Node, a *List, or nil.
func (n *Node) Set(key string, value any) *Node {
	for i, k := range n.keys {
		if k == key {
			n.values[i] = value
			return n
		}
	}
	n.keys = append(n.keys, key)
	n.values = append(n.values, value)
	return n
}

// SetString gives a field a string value.
func (n *Node) SetString(key string, value string) *Node {
	return n.Set(key, jsutil.FromString(value))
}

// Has reports whether the field is there at all.
func (n *Node) Has(key string) bool {
	for _, k := range n.keys {
		if k == key {
			return true
		}
	}
	return false
}

// Get is the value of a field, or nil when it is not there.
func (n *Node) Get(key string) any {
	for i, k := range n.keys {
		if k == key {
			return n.values[i]
		}
	}
	return nil
}

// Keys are the field names in their order, without type and loc.
func (n *Node) Keys() []string {
	return n.keys
}

// Str is the value of a string field, or nil.
func (n *Node) Str(key string) jsutil.Str {
	if v, ok := n.Get(key).(jsutil.Str); ok {
		return v
	}
	return nil
}

// StrOrEmpty is the value of a string field as a Go string.
func (n *Node) StrOrEmpty(key string) string {
	return n.Str(key).String()
}

// Bool is the value of a boolean field.
func (n *Node) Bool(key string) bool {
	v, _ := n.Get(key).(bool)
	return v
}

// Child is the value of a field that holds one node, or nil.
func (n *Node) Child(key string) *Node {
	if v, ok := n.Get(key).(*Node); ok {
		return v
	}
	return nil
}

// Children is the value of a field that holds a list, or nil.
func (n *Node) Children(key string) *List {
	if v, ok := n.Get(key).(*List); ok {
		return v
	}
	return nil
}

// List is the children list of a node that has one.
func (n *Node) List() *List {
	return n.Children("children")
}
