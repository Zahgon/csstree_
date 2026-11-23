// Package syntaxdef holds the value definition syntax: the notation a CSS specification writes a
// value grammar in, such as `<length> | auto`, `[ a | b ]{1,4}` or `rgb( <number>#{3} )`. It reads
// a definition into a tree, writes a tree back out as a definition, and walks a tree.
//
// A node keeps its own fields in the order they were set, because that order is the order the
// tree is written out in as JSON, and it is the order the original's object literals have.
package syntaxdef

import "github.com/csstree/csstree/internal/jsutil"

// Node is one node of a value definition syntax tree. Besides its type it carries named fields,
// which are a string, a boolean, a number, a child node, a list of terms, or nothing.
type Node struct {
	Type string

	keys   []string
	values []any
}

// New returns a node of the type with no fields set.
func New(nodeType string) *Node {
	return &Node{Type: nodeType}
}

// Set gives a field a value, keeping the position a field already there has, and returns the node.
// The value is a jsutil.Str, a bool, a float64, a *Node, a []*Node, a jsutil.Value, or nil.
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

// Get is the value of a field, or nil when it is not there.
func (n *Node) Get(key string) any {
	for i, k := range n.keys {
		if k == key {
			return n.values[i]
		}
	}
	return nil
}

// Keys are the field names in their order, without type.
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

// Bool is the value of a boolean field.
func (n *Node) Bool(key string) bool {
	v, _ := n.Get(key).(bool)
	return v
}

// Num is the value of a numeric field.
func (n *Node) Num(key string) float64 {
	v, _ := n.Get(key).(float64)
	return v
}

// Child is the value of a field that holds one node, or nil.
func (n *Node) Child(key string) *Node {
	if v, ok := n.Get(key).(*Node); ok {
		return v
	}
	return nil
}

// Terms is the value of a field that holds a list of nodes, or nil.
func (n *Node) Terms(key string) []*Node {
	if v, ok := n.Get(key).([]*Node); ok {
		return v
	}
	return nil
}
