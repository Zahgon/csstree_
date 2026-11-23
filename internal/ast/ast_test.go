package ast

import (
	"testing"

	"github.com/csstree/csstree/internal/jsutil"
)

// TestFieldOrderIsKept checks that a node writes its fields in the order they were set, which is
// what the JSON of a tree depends on.
func TestFieldOrderIsKept(t *testing.T) {
	node := New("Declaration")
	node.Set("important", false)
	node.SetString("property", "color")
	node.Set("value", New("Raw").SetString("value", "red"))

	got := jsutil.Stringify(ToPlain(node), 0).String()
	want := `{"type":"Declaration","loc":null,"important":false,"property":"color","value":{"type":"Raw","loc":null,"value":"red"}}`

	if got != want {
		t.Errorf("the tree writes as\n %s\nwant\n %s", got, want)
	}

	// Setting a field again keeps the place it already had.
	node.SetString("property", "background")
	if got := jsutil.Stringify(ToPlain(node), 0).String(); got != `{"type":"Declaration","loc":null,"important":false,"property":"background","value":{"type":"Raw","loc":null,"value":"red"}}` {
		t.Errorf("setting a field again moved it: %s", got)
	}
}

// TestRoundTrip checks that a tree survives being turned into a plain value and back.
func TestRoundTrip(t *testing.T) {
	tree := New("StyleSheet")
	children := NewList()
	for _, name := range []string{"a", "b"} {
		children.AppendData(New("Identifier").SetString("name", name))
	}
	tree.Set("children", children)
	tree.Loc = &Location{Source: jsutil.FromString("x.css"),
		Start: Position{Offset: 0, Line: 1, Column: 1},
		End:   Position{Offset: 2, Line: 1, Column: 3}}

	before := jsutil.Stringify(ToPlain(tree), 0).String()
	after := jsutil.Stringify(ToPlain(FromPlain(ToPlain(tree))), 0).String()

	if before != after {
		t.Errorf("a round trip changed the tree:\n %s\n %s", before, after)
	}
}

// TestClone checks that a copy is a copy: changing it leaves the tree it came from alone.
func TestClone(t *testing.T) {
	tree := New("Value")
	children := NewList()
	children.AppendData(New("Identifier").SetString("name", "a"))
	tree.Set("children", children)

	copied := Clone(tree)
	copied.List().First().SetString("name", "b")

	if got := tree.List().First().StrOrEmpty("name"); got != "a" {
		t.Errorf("the original now reads %q", got)
	}
	if got := copied.List().First().StrOrEmpty("name"); got != "b" {
		t.Errorf("the copy reads %q", got)
	}
}

// TestListOperations checks the list a node holds its children in.
func TestListOperations(t *testing.T) {
	list := NewList()
	first := list.AppendData(New("Identifier").SetString("name", "a"))
	list.AppendData(New("Identifier").SetString("name", "b"))
	list.PrependData(New("Identifier").SetString("name", "z"))

	if got := list.Len(); got != 3 {
		t.Errorf("the list holds %d items, want 3", got)
	}
	if got := list.First().StrOrEmpty("name"); got != "z" {
		t.Errorf("the first item is %q, want z", got)
	}
	if got := list.Last().StrOrEmpty("name"); got != "b" {
		t.Errorf("the last item is %q, want b", got)
	}

	list.Remove(first)
	if got := list.Len(); got != 2 {
		t.Errorf("after a removal the list holds %d items, want 2", got)
	}

	var seen []string
	list.Each(func(node *Node, item *Item, l *List) {
		seen = append(seen, node.StrOrEmpty("name"))
	})
	if len(seen) != 2 || seen[0] != "z" || seen[1] != "b" {
		t.Errorf("the walk saw %v", seen)
	}

	var reversed []string
	list.EachRight(func(node *Node, item *Item, l *List) {
		reversed = append(reversed, node.StrOrEmpty("name"))
	})
	if len(reversed) != 2 || reversed[0] != "b" || reversed[1] != "z" {
		t.Errorf("the backward walk saw %v", reversed)
	}
}

// TestTypesTable checks that every node type the parser builds has a declared structure.
func TestTypesTable(t *testing.T) {
	if len(TypeNames) < 40 {
		t.Errorf("only %d node types are declared", len(TypeNames))
	}

	for _, name := range TypeNames {
		if Types[name] == nil {
			t.Errorf("%s has no declared structure", name)
		}
	}

	declaration := Types["Declaration"]
	if declaration == nil {
		t.Fatal("Declaration has no declared structure")
	}
	if got, want := len(declaration.Fields), 3; got != want {
		t.Errorf("Declaration declares %d fields, want %d", got, want)
	}
	if declaration.WalkContext != "declaration" {
		t.Errorf("the walk context of Declaration is %q", declaration.WalkContext)
	}
}
