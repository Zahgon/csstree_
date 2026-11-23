// Package walker visits every node of a tree. It reads the declared structure of each node type
// to know which fields hold children, keeps the nine walk contexts up to date while it descends,
// and offers the find helpers the original exposes beside the walk itself.
package walker

import (
	"strings"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
)

// Context is what a walk tells a callback about where it is. Root is the node the walk started
// from, and each other field is the closest node of that kind the walk is inside of, or nil.
type Context struct {
	Root          *ast.Node
	StyleSheet    *ast.Node
	Atrule        *ast.Node
	AtrulePrelude *ast.Node
	Rule          *ast.Node
	Selector      *ast.Node
	Block         *ast.Node
	Declaration   *ast.Node
	Function      *ast.Node
}

// field is the context the name of a walk context stands for, or nil when there is no such field.
func (c *Context) field(name string) **ast.Node {
	switch name {
	case "stylesheet":
		return &c.StyleSheet
	case "atrule":
		return &c.Atrule
	case "atrulePrelude":
		return &c.AtrulePrelude
	case "rule":
		return &c.Rule
	case "selector":
		return &c.Selector
	case "block":
		return &c.Block
	case "declaration":
		return &c.Declaration
	case "function":
		return &c.Function
	}
	return nil
}

// Visitor is called for every node a walk reaches. item and list are the place the node sits in
// when it is a child of a list, and nil when it is held by a named field.
type Visitor func(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context)

// Options steers a walk: the callbacks to call, the one node type to call them for, and whether
// to visit the children of a node back to front.
type Options struct {
	Enter   Visitor
	Leave   Visitor
	Visit   string
	Reverse bool
}

// Predicate answers whether a node is the one a find is after.
type Predicate func(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context) bool

// What a callback leaves a walk to do next.
const (
	walkContinue = iota
	walkBreak
	walkSkip
)

// visitor is a callback inside the walk, which unlike the public one may stop the walk or skip
// the children of the node it was given.
type visitor func(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context) int

// walkerField is one field of a node type that holds children.
type walkerField struct {
	name     string
	isList   bool
	nullable bool
}

// typeWalkers is what a walk has to do for one node type: the walk context it sets while it is
// inside such a node, and the fields it descends into.
type typeWalkers struct {
	context string
	fields  []walkerField
}

// iterator descends into the children of one node, and reports whether the walk was stopped.
type iterator func(node *ast.Node, ctx *Context, walk func(*ast.Node, *ast.Item, *ast.List) bool) bool

var (
	// types is every node type, with nil for a type that holds no children at all.
	types = map[string]*typeWalkers{}

	iteratorsNatural = map[string]iterator{}
	iteratorsReverse = map[string]iterator{}

	fastTraversalNatural = map[string]map[string]iterator{}
	fastTraversalReverse = map[string]map[string]iterator{}
)

func init() {
	for name, nodeType := range ast.Types {
		types[name] = getWalkersFromStructure(nodeType)
	}

	for name, walkers := range types {
		if walkers == nil {
			continue
		}
		iteratorsNatural[name] = createTypeIterator(walkers, false)
		iteratorsReverse[name] = createTypeIterator(walkers, true)
	}

	fastTraversalNatural = createFastTraversalMap(iteratorsNatural)
	fastTraversalReverse = createFastTraversalMap(iteratorsReverse)
}

// getWalkersFromStructure reads the declared structure of a node type and keeps the fields that
// hold a node or a list of nodes. A type with no such field has nothing to walk.
func getWalkersFromStructure(nodeType *ast.TypeDef) *typeWalkers {
	var walkers []walkerField

	for _, field := range nodeType.Fields {
		walker := walkerField{name: field.Name, nullable: field.Nullable}

		switch {
		case field.IsList:
			walker.isList = true
		case len(field.NodeTypes) > 0:
			walker.isList = false
		default:
			continue
		}

		walkers = append(walkers, walker)
	}

	if len(walkers) > 0 {
		return &typeWalkers{context: nodeType.WalkContext, fields: walkers}
	}

	return nil
}

// createTypeIterator builds the step that descends into one node type. In reverse the fields are
// taken back to front, as are the children of every list.
func createTypeIterator(cfg *typeWalkers, reverse bool) iterator {
	fields := append([]walkerField(nil), cfg.fields...)
	contextName := cfg.context
	useContext := contextName != ""

	if reverse {
		for i, j := 0, len(fields)-1; i < j; i, j = i+1, j-1 {
			fields[i], fields[j] = fields[j], fields[i]
		}
	}

	return func(node *ast.Node, ctx *Context, walk func(*ast.Node, *ast.Item, *ast.List) bool) bool {
		var prevContextValue *ast.Node
		var slot **ast.Node

		if useContext {
			slot = ctx.field(contextName)
			prevContextValue = *slot
			*slot = node
		}

		for _, field := range fields {
			ref := node.Get(field.name)

			if field.nullable && !truthy(ref) {
				continue
			}

			if field.isList {
				list, _ := ref.(*ast.List)
				broken := false
				step := func(child *ast.Node, item *ast.Item, l *ast.List) {
					if !broken {
						broken = walk(child, item, l)
					}
				}

				if reverse {
					list.EachRight(step)
				} else {
					list.Each(step)
				}

				if broken {
					// The original leaves the walk context as it is when a walk is stopped.
					return true
				}
			} else if child, ok := ref.(*ast.Node); ok && child != nil {
				if walk(child, nil, nil) {
					return true
				}
			}
		}

		if useContext {
			*slot = prevContextValue
		}

		return false
	}
}

// createFastTraversalMap is the shortcut the original takes when a walk is only after atrules,
// rules or declarations: it descends only through the node types those can be nested in.
func createFastTraversalMap(iterators map[string]iterator) map[string]map[string]iterator {
	return map[string]map[string]iterator{
		"Atrule": {
			"StyleSheet": iterators["StyleSheet"],
			"Atrule":     iterators["Atrule"],
			"Rule":       iterators["Rule"],
			"Block":      iterators["Block"],
		},
		"Rule": {
			"StyleSheet": iterators["StyleSheet"],
			"Atrule":     iterators["Atrule"],
			"Rule":       iterators["Rule"],
			"Block":      iterators["Block"],
		},
		"Declaration": {
			"StyleSheet":      iterators["StyleSheet"],
			"Atrule":          iterators["Atrule"],
			"Rule":            iterators["Rule"],
			"Block":           iterators["Block"],
			"DeclarationList": iterators["DeclarationList"],
		},
	}
}

// truthy is the JavaScript truth of a field value, which is what decides whether a nullable field
// is descended into.
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case jsutil.Str:
		return len(v) > 0
	case bool:
		return v
	case *ast.Node:
		return v != nil
	case *ast.List:
		return v != nil
	}
	return true
}

// Walk visits every node under root, calling Enter on the way in and Leave on the way out. It
// returns the error the original throws for a bad Visit, or for a walk with no callback at all.
func Walk(root *ast.Node, opts Options) *jsutil.Error {
	return run(root, opts.Visit, opts.Reverse, wrap(opts.Enter), wrap(opts.Leave))
}

// wrap turns a callback that only looks at a node into one the walk can read an answer from.
func wrap(fn Visitor) visitor {
	if fn == nil {
		return nil
	}
	return func(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context) int {
		fn(node, item, list, ctx)
		return walkContinue
	}
}

// invokeForType narrows a callback to one node type.
func invokeForType(fn visitor, nodeType string) visitor {
	return func(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context) int {
		if node != nil && node.Type == nodeType {
			return fn(node, item, list, ctx)
		}
		return walkContinue
	}
}

// noop is the callback a walk uses where none was given.
func noop(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context) int {
	return walkContinue
}

// run is the walk itself. enter and leave are nil when there is none.
func run(root *ast.Node, visit string, reverse bool, enter, leave visitor) *jsutil.Error {
	enterIsNoop := enter == nil
	leaveIsNoop := leave == nil

	if enterIsNoop {
		enter = noop
	}
	if leaveIsNoop {
		leave = noop
	}

	iterators := iteratorsNatural
	if reverse {
		iterators = iteratorsReverse
	}

	if visit != "" {
		if fast, ok := fastTraversalNatural[visit]; ok {
			if reverse {
				iterators = fastTraversalReverse[visit]
			} else {
				iterators = fast
			}
		} else if _, ok := types[visit]; !ok {
			return jsutil.NewError("Error", jsutil.FromString(
				"Bad value `"+visit+"` for `visit` option (should be: "+strings.Join(ast.TypeNames, ", ")+")"))
		}

		// Narrowing a callback leaves a function behind even where there was none, so a walk that
		// names a type is never the walk with no callback at all.
		enter = invokeForType(enter, visit)
		leave = invokeForType(leave, visit)
		enterIsNoop = false
		leaveIsNoop = false
	}

	if enterIsNoop && leaveIsNoop {
		return jsutil.NewError("Error", jsutil.FromString(
			"Neither `enter` nor `leave` walker handler is set or both aren't a function"))
	}

	ctx := &Context{Root: root}

	var walkNode func(node *ast.Node, item *ast.Item, list *ast.List) bool
	walkNode = func(node *ast.Node, item *ast.Item, list *ast.List) bool {
		switch enter(node, item, list, ctx) {
		case walkBreak:
			return true
		case walkSkip:
			return false
		}

		if node != nil {
			if it, ok := iterators[node.Type]; ok && it != nil {
				if it(node, ctx, walkNode) {
					return true
				}
			}
		}

		return leave(node, item, list, ctx) == walkBreak
	}

	walkNode(root, nil, nil)

	return nil
}

// Find is the first node under root the predicate answers for, or nil.
func Find(root *ast.Node, fn Predicate) *ast.Node {
	var found *ast.Node

	run(root, "", false, func(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context) int {
		if fn(node, item, list, ctx) {
			found = node
			return walkBreak
		}
		return walkContinue
	}, nil)

	return found
}

// FindLast is the last node under root the predicate answers for, or nil. It walks the tree back
// to front and stops at the first answer.
func FindLast(root *ast.Node, fn Predicate) *ast.Node {
	var found *ast.Node

	run(root, "", true, func(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context) int {
		if fn(node, item, list, ctx) {
			found = node
			return walkBreak
		}
		return walkContinue
	}, nil)

	return found
}

// FindAll is every node under root the predicate answers for, in the order the walk reaches them.
func FindAll(root *ast.Node, fn Predicate) []*ast.Node {
	found := []*ast.Node{}

	run(root, "", false, func(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context) int {
		if fn(node, item, list, ctx) {
			found = append(found, node)
		}
		return walkContinue
	}, nil)

	return found
}
