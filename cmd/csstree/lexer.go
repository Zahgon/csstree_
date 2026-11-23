package main

import (
	"fmt"
	"strings"

	csstree "github.com/csstree/csstree"
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/lexer"
	"github.com/csstree/csstree/internal/parser"
	"github.com/csstree/csstree/internal/syntaxdef"
	"github.com/csstree/csstree/internal/walker"
)

func init() {
	commands["match"] = cmdMatch
	commands["check"] = cmdCheck
	commands["check-structure"] = cmdCheckStructure
	commands["syntax"] = cmdSyntax
}

// matchTreeValue writes a match tree the way the original writes its matched member.
func matchTreeValue(tree *lexer.MatchTree) jsutil.Value {
	object := jsutil.NewObject()

	if tree.Syntax == nil {
		object.SetString("syntax", jsutil.Null{})
	} else {
		object.SetString("syntax", syntaxdef.ToPlain(tree.Syntax))
	}

	if tree.IsLeaf {
		object.SetString("token", jsutil.JSONString(tree.Token))
		if tree.Node == nil {
			object.SetString("node", jsutil.Null{})
		} else {
			object.SetString("node", ast.ToPlain(tree.Node))
		}
		return object
	}

	items := jsutil.Array{}
	for _, child := range tree.Match {
		items = append(items, matchTreeValue(child))
	}
	object.SetString("match", items)

	return object
}

func parseValue(text jsutil.Str, context string) (*ast.Node, *jsutil.Error) {
	options := parser.NewOptions()
	options.Context = context
	return parser.Parse(text, options)
}

func cmdMatch(o *output, a *args) (int, error) {
	l, err := csstree.DefaultLexer()
	if err != nil {
		return 0, err
	}

	kind := ""
	var names []jsutil.Str
	var value jsutil.Str
	hasValue := false
	trace := false
	var fragments []string

	take := func(flag string, n int) error {
		names = names[:0]
		for i := 0; i < n; i++ {
			v, err := a.value(flag)
			if err != nil {
				return err
			}
			names = append(names, jsutil.FromString(v))
		}
		return nil
	}

	if err := a.parseFlags(func(flag string) (bool, error) {
		switch flag {
		case "--property", "--type", "--atrule-prelude", "--syntax":
			kind = strings.TrimPrefix(flag, "--")
			return true, take(flag, 1)
		case "--atrule-descriptor":
			kind = "atrule-descriptor"
			return true, take(flag, 2)
		case "--declaration":
			kind = "declaration"
		case "--value":
			v, err := a.value(flag)
			if err != nil {
				return true, err
			}
			value, hasValue = jsutil.FromString(v), true
		case "--trace":
			trace = true
		case "--fragments":
			first, err := a.value(flag)
			if err != nil {
				return true, err
			}
			second, err := a.value(flag)
			if err != nil {
				return true, err
			}
			fragments = []string{first, second}
		default:
			return false, nil
		}
		return true, nil
	}); err != nil {
		return 0, err
	}

	if kind == "" {
		return 0, usagef("one of --property, --type, --atrule-prelude, --atrule-descriptor, --declaration or --syntax is required")
	}

	if !hasValue {
		name, present := a.optional()
		text, err := readText(name, present)
		if err != nil {
			return 0, err
		}
		value = text
	}
	if err := a.end(); err != nil {
		return 0, err
	}

	if fragments != nil && kind != "property" {
		return 0, usagef("--fragments needs --property")
	}

	// With --trace or --fragments the value is parsed first and the parsed tree is matched, so
	// that the trace and the fragments name nodes of that tree.
	asTree := trace || fragments != nil

	var result *lexer.MatchResult
	var valueAst *ast.Node

	makeValue := func(context string) (lexer.Value, *jsutil.Error) {
		if !asTree {
			return lexer.TextValue(value), nil
		}
		tree, err := parseValue(value, context)
		if err != nil {
			return lexer.Value{}, err
		}
		valueAst = tree
		return lexer.NodeValue(tree), nil
	}

	switch kind {
	case "property":
		v, err := makeValue("value")
		if err != nil {
			return 0, err
		}
		result = l.MatchProperty(names[0], v)

	case "type":
		v, err := makeValue("value")
		if err != nil {
			return 0, err
		}
		result = l.MatchType(names[0], v)

	case "atrule-prelude":
		v, err := makeValue("atrulePrelude")
		if err != nil {
			return 0, err
		}
		result = l.MatchAtrulePrelude(names[0], v)

	case "atrule-descriptor":
		v, err := makeValue("value")
		if err != nil {
			return 0, err
		}
		result = l.MatchAtruleDescriptor(names[0], names[1], v)

	case "syntax":
		v, err := makeValue("value")
		if err != nil {
			return 0, err
		}
		result = l.Match(names[0], v)

	case "declaration":
		declaration, err := parseValue(value, "declaration")
		if err != nil {
			return 0, err
		}
		valueAst = declaration.Child("value")
		result = l.MatchDeclaration(declaration)
	}

	if result.Matched == nil {
		if result.Error == nil {
			// An at-rule with no prelude syntax: nothing matched and nothing failed.
			o.writeErr("NO-SYNTAX\n")
			if flushErr := o.flush(); flushErr != nil {
				return 0, flushErr
			}
			return 1, nil
		}
		return 0, result.Error
	}

	o.writeString("MATCHED\n")
	o.write(jsonOf(matchTreeValue(result.Matched), 1))
	o.writeString("\n")

	if trace {
		walker.Walk(valueAst, walker.Options{
			Enter: func(node *ast.Node, _ *ast.Item, _ *ast.List, _ *walker.Context) {
				traceList, found := result.GetTrace(node)
				var traceValue jsutil.Value = jsutil.Null{}
				if found {
					items := jsutil.Array{}
					for _, syntax := range traceList {
						items = append(items, syntaxdef.ToPlain(syntax))
					}
					traceValue = items
				}
				o.writeString("TRACE\t" + jsonString(generateNode(node)).String() + "\t" +
					jsonOf(traceValue, 0).String() + "\n")
			},
		})
	}

	if fragments != nil {
		for _, fragment := range lexer.MatchFragments(valueAst, result, fragments[0], fragments[1]) {
			text := jsutil.Str{}
			for _, node := range fragment.Nodes {
				text = append(text, generateNode(node)...)
			}
			o.writeString("FRAGMENT\t" + jsonString(text).String() + "\n")
		}
	}

	return 0, nil
}

func cmdCheck(o *output, a *args) (int, error) {
	l, err := csstree.DefaultLexer()
	if err != nil {
		return 0, err
	}

	var reported *jsutil.Error
	done := false

	if err := a.parseFlags(func(flag string) (bool, error) {
		read := func(n int) ([]jsutil.Str, error) {
			out := make([]jsutil.Str, 0, n)
			for i := 0; i < n; i++ {
				v, err := a.value(flag)
				if err != nil {
					return nil, err
				}
				out = append(out, jsutil.FromString(v))
			}
			return out, nil
		}

		switch flag {
		case "--property-name":
			values, err := read(1)
			if err != nil {
				return true, err
			}
			reported, done = l.CheckPropertyName(values[0]), true
		case "--atrule-name":
			values, err := read(1)
			if err != nil {
				return true, err
			}
			reported, done = l.CheckAtruleName(values[0]), true
		case "--atrule-prelude":
			values, err := read(2)
			if err != nil {
				return true, err
			}
			reported, done = l.CheckAtrulePrelude(values[0], values[1], true), true
		case "--atrule-descriptor":
			values, err := read(2)
			if err != nil {
				return true, err
			}
			reported, done = l.CheckAtruleDescriptorName(values[0], values[1]), true
		default:
			return false, nil
		}
		return true, nil
	}); err != nil {
		return 0, err
	}

	if err := a.end(); err != nil {
		return 0, err
	}

	if !done {
		return 0, usagef("one of --property-name, --atrule-name, --atrule-prelude or --atrule-descriptor is required")
	}

	if reported != nil {
		return 0, reported
	}

	o.writeString("OK\n")
	return 0, nil
}

func cmdCheckStructure(o *output, a *args) (int, error) {
	if err := a.parseFlags(func(string) (bool, error) { return false, nil }); err != nil {
		return 0, err
	}
	name, present := a.optional()
	value, err := readJSON(name, present)
	if err != nil {
		return 0, err
	}
	if err := a.end(); err != nil {
		return 0, err
	}

	warnings := lexer.CheckStructure(lexer.ToListTree(value))

	if warnings == nil {
		o.writeString("OK\n")
		return 0, nil
	}

	for _, warning := range warnings {
		nodeType := warning.NodeType
		if !warning.HasType {
			nodeType = "undefined"
		}
		o.writeString(fmt.Sprintf("STRUCTURE\t%s\t%s\n", nodeType, warning.Message))
	}

	return 1, nil
}

func cmdSyntax(o *output, a *args) (int, error) {
	l, err := csstree.DefaultLexer()
	if err != nil {
		return 0, err
	}

	sub, ok := a.next()
	if !ok {
		return 0, usagef("unknown syntax sub-command (none)")
	}

	switch sub {
	case "get":
		kind := ""
		var name jsutil.Str
		compact := false

		if err := a.parseFlags(func(flag string) (bool, error) {
			switch flag {
			case "--property", "--type", "--atrule":
				kind = strings.TrimPrefix(flag, "--")
				v, err := a.value(flag)
				if err != nil {
					return true, err
				}
				name = jsutil.FromString(v)
			case "--compact":
				compact = true
			default:
				return false, nil
			}
			return true, nil
		}); err != nil {
			return 0, err
		}
		if err := a.end(); err != nil {
			return 0, err
		}

		var descriptor *lexer.Descriptor
		switch kind {
		case "property":
			descriptor = l.GetProperty(name, true)
		case "type":
			descriptor = l.GetType(name)
		case "atrule":
			descriptor = l.GetAtrulePrelude(name, true)
		default:
			return 0, usagef("one of --property, --type or --atrule is required")
		}

		if descriptor == nil {
			o.writeString("# none\n")
			return 1, nil
		}

		definition, defErr := descriptor.Syntax()
		if defErr != nil {
			return 0, defErr
		}

		o.write(syntaxdef.Generate(definition, false, compact))
		o.writeString("\n")
		return 0, nil

	case "dump":
		compact := false
		if err := a.parseFlags(func(flag string) (bool, error) {
			if flag == "--compact" {
				compact = true
				return true, nil
			}
			return false, nil
		}); err != nil {
			return 0, err
		}
		if err := a.end(); err != nil {
			return 0, err
		}

		o.write(jsonOf(l.Dump(false, compact), 2))
		o.writeString("\n")
		return 0, nil

	case "validate":
		if err := a.parseFlags(func(string) (bool, error) { return false, nil }); err != nil {
			return 0, err
		}
		if err := a.end(); err != nil {
			return 0, err
		}

		o.write(jsonOf(l.Validate(), 2))
		o.writeString("\n")
		return 0, nil
	}

	return 0, usagef("unknown syntax sub-command %s", sub)
}
