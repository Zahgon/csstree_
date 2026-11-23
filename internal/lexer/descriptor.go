package lexer

import (
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/syntaxdef"
)

// Descriptor is one entry of the dictionary: a property, a type, an at-rule prelude or an at-rule
// descriptor, with the definition it was written as and the match graph built from it. Both are
// built when they are first asked for, as the original builds them, so a definition that cannot
// be read only fails where it is used.
type Descriptor struct {
	Type   string
	Name   string
	Parent string

	// Serializable says whether the definition can be written back out, which it can when it was
	// given as text or as a definition tree.
	Serializable bool

	text    jsutil.Str
	hasText bool
	syntax  *syntaxdef.Node
	generic genericFn
	ref     *syntaxdef.Node

	parsed     bool
	parseError *jsutil.Error
	match      *matchNode
	matchRef   *matchNode
	matchBuilt bool
}

// newDescriptor builds a descriptor from a definition written as text.
func newDescriptor(text jsutil.Str, kind, name, parent string) *Descriptor {
	return &Descriptor{
		Type:         kind,
		Name:         name,
		Parent:       parent,
		Serializable: true,
		text:         text,
		hasText:      true,
		ref:          referenceNode(kind, name),
	}
}

// newTreeDescriptor builds a descriptor from a definition already read into a tree.
func newTreeDescriptor(syntax *syntaxdef.Node, kind, name, parent string) *Descriptor {
	return &Descriptor{
		Type:         kind,
		Name:         name,
		Parent:       parent,
		Serializable: syntax != nil && syntax.Type != "",
		syntax:       syntax,
		parsed:       true,
		ref:          referenceNode(kind, name),
	}
}

// newGenericDescriptor builds a descriptor for a type the lexer matches with code of its own.
func newGenericDescriptor(fn genericFn, kind, name string) *Descriptor {
	d := &Descriptor{
		Type:    kind,
		Name:    name,
		generic: fn,
		parsed:  true,
		ref:     referenceNode(kind, name),
	}
	d.match = buildGenericMatchGraph(fn, d.ref)
	d.matchBuilt = true
	return d
}

// referenceNode names the entry a match tree points back at.
func referenceNode(kind, name string) *syntaxdef.Node {
	node := syntaxdef.New(kind)
	node.Set("name", jsutil.FromString(name))
	return node
}

// Syntax is the definition tree, read from the text the first time it is asked for.
func (d *Descriptor) Syntax() (*syntaxdef.Node, *jsutil.Error) {
	if !d.parsed {
		d.parsed = true
		d.syntax, d.parseError = syntaxdef.Parse(d.text)
	}
	return d.syntax, d.parseError
}

// Match is the graph the matcher walks, built the first time it is asked for. Building it reads a
// definition tree, which unwinds by raising when it meets a node type it does not know; that is
// caught here, so the package reports it as an error value like any other.
func (d *Descriptor) Match() (node *matchNode, err *jsutil.Error) {
	if d.matchBuilt {
		return d.match, d.parseError
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			raised, ok := recovered.(*jsutil.Error)
			if !ok {
				panic(recovered)
			}
			d.matchBuilt = true
			d.match = nil
			d.parseError = raised
			node, err = nil, raised
		}
	}()

	syntax, parseErr := d.Syntax()
	if parseErr != nil {
		return nil, parseErr
	}

	d.matchBuilt = true
	d.match = buildMatchGraph(syntax, d.ref)

	if d.Type == "Property" && syntaxHasTopLevelCommaMultiplier(syntax) {
		// A property whose whole definition repeats with commas is referred to by one of its
		// terms when another definition names it, so a graph of that one term is kept as well.
		terms := syntax.Terms("terms")
		single := syntaxdef.New("Group")
		for _, key := range syntax.Keys() {
			if key == "terms" {
				single.Set("terms", []*syntaxdef.Node{terms[0].Child("term")})
				continue
			}
			single.Set(key, syntax.Get(key))
		}
		d.matchRef = buildMatchGraph(single, d.ref)
	}

	return d.match, nil
}

// syntaxHasTopLevelCommaMultiplier reports whether a definition is one term repeated with commas.
func syntaxHasTopLevelCommaMultiplier(syntax *syntaxdef.Node) bool {
	if syntax == nil || syntax.Type != "Group" {
		return false
	}

	terms := syntax.Terms("terms")

	return !syntax.Bool("explicit") &&
		len(terms) == 1 &&
		terms[0].Type == "Multiplier" &&
		terms[0].Bool("comma")
}
