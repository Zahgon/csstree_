package parser

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
)

// The fixtures of the repository: a directory per parse context, each file mapping a case name to
// the CSS source, the tree it is expected to read as, and sometimes the options it is read with.
const fixtureRoot = "../../fixtures/ast"

// wrappers are the trees the fixture loader of the original wraps a case in when the case names a
// node type that is not the one its directory stands for.
var wrappers = map[string]func(jsutil.Value) jsutil.Value{
	"stylesheet": func(tree jsutil.Value) jsutil.Value {
		return jsutil.NewObject().
			SetString("type", jsutil.JSONString(jsutil.FromString("StyleSheet"))).
			SetString("children", jsutil.Array{tree})
	},
	"mediaQuery": func(tree jsutil.Value) jsutil.Value {
		return jsutil.NewObject().
			SetString("type", jsutil.JSONString(jsutil.FromString("MediaQuery"))).
			SetString("modifier", jsutil.Null{}).
			SetString("mediaType", jsutil.Null{}).
			SetString("condition", tree)
	},
	"rule": func(tree jsutil.Value) jsutil.Value {
		return jsutil.NewObject().
			SetString("type", jsutil.JSONString(jsutil.FromString("Rule"))).
			SetString("prelude", jsutil.NewObject().
				SetString("type", jsutil.JSONString(jsutil.FromString("SelectorList"))).
				SetString("children", jsutil.Array{})).
			SetString("block", tree)
	},
	"selector": func(tree jsutil.Value) jsutil.Value {
		return jsutil.NewObject().
			SetString("type", jsutil.JSONString(jsutil.FromString("Selector"))).
			SetString("children", jsutil.Array{tree})
	},
	"value": func(tree jsutil.Value) jsutil.Value {
		return jsutil.NewObject().
			SetString("type", jsutil.JSONString(jsutil.FromString("Value"))).
			SetString("children", jsutil.Array{tree})
	},
}

type fixtureCase struct {
	name    string
	scope   string
	source  jsutil.Str
	tree    jsutil.Value
	options *jsutil.Object
	isError bool
}

func loadFixtures(t *testing.T) []fixtureCase {
	t.Helper()

	scopes, err := os.ReadDir(fixtureRoot)
	if err != nil {
		t.Fatalf("reading the fixtures: %v", err)
	}

	cases := []fixtureCase{}

	for _, scopeEntry := range scopes {
		if !scopeEntry.IsDir() {
			continue
		}
		scope := scopeEntry.Name()

		files := []string{}
		err := filepath.Walk(filepath.Join(fixtureRoot, scope), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".json") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("reading the fixtures of %s: %v", scope, err)
		}
		sort.Strings(files)

		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("reading %s: %v", file, err)
			}
			value, err := jsutil.ParseJSONBytes(raw)
			if err != nil {
				t.Fatalf("reading %s: %v", file, err)
			}
			object, ok := value.(*jsutil.Object)
			if !ok {
				continue
			}

			for _, key := range object.Keys() {
				entry, _ := object.Get(key)
				items := jsutil.Array{entry}
				if list, isList := entry.(jsutil.Array); isList {
					items = list
				}

				for i, item := range items {
					fields, ok := item.(*jsutil.Object)
					if !ok {
						continue
					}
					source, ok := fields.Get(jsutil.FromString("source"))
					if !ok {
						continue
					}
					text, ok := source.(jsutil.JSONString)
					if !ok {
						continue
					}

					name := file + ":" + key.String()
					if len(items) > 1 {
						name += "#" + strconv.Itoa(i)
					}

					c := fixtureCase{name: name, scope: scope, source: jsutil.Str(text)}

					if _, isError := fields.Get(jsutil.FromString("error")); isError {
						c.isError = true
					}
					if tree, ok := fields.Get(jsutil.FromString("ast")); ok {
						c.tree = tree
					}
					if options, ok := fields.Get(jsutil.FromString("options")); ok {
						if o, isObject := options.(*jsutil.Object); isObject {
							c.options = o
						}
					}

					cases = append(cases, c)
				}
			}
		}
	}

	if len(cases) == 0 {
		t.Fatal("no fixtures were found")
	}

	return cases
}

// optionsOf builds the parse options a fixture asks for.
func optionsOf(c fixtureCase) Options {
	options := NewOptions()
	options.Context = c.scope

	if c.options == nil {
		return options
	}

	read := func(key string) (jsutil.Value, bool) {
		return c.options.Get(jsutil.FromString(key))
	}

	if value, ok := read("parseValue"); ok {
		options.ParseValue = value == jsutil.Bool(true)
	}
	if value, ok := read("parseAtrulePrelude"); ok {
		options.ParseAtrulePrelude = value == jsutil.Bool(true)
	}
	if value, ok := read("parseRulePrelude"); ok {
		options.ParseRulePrelude = value == jsutil.Bool(true)
	}
	if value, ok := read("parseCustomProperty"); ok {
		options.ParseCustomProperty = value == jsutil.Bool(true)
	}
	if value, ok := read("context"); ok {
		if text, isText := value.(jsutil.JSONString); isText {
			options.Context = jsutil.Str(text).String()
		}
	}
	if value, ok := read("atrule"); ok {
		if text, isText := value.(jsutil.JSONString); isText {
			options.Context = "atrulePrelude"
			options.Atrule = jsutil.Str(text)
		}
	}

	return options
}

// withoutLoc drops every loc from a tree, since a fixture states the shape of a tree rather than
// the places it came from.
func withoutLoc(value jsutil.Value) jsutil.Value {
	switch v := value.(type) {
	case *jsutil.Object:
		out := jsutil.NewObject()
		for _, key := range v.Keys() {
			if key.EqualString("loc") {
				continue
			}
			field, _ := v.Get(key)
			out.Set(key, withoutLoc(field))
		}
		return out
	case jsutil.Array:
		items := make(jsutil.Array, len(v))
		for i, item := range v {
			items[i] = withoutLoc(item)
		}
		return items
	}
	return value
}

// TestFixtureTrees parses every fixture source in its context and compares the tree with the one
// the fixture states.
func TestFixtureTrees(t *testing.T) {
	passed, failed := 0, 0

	for _, c := range loadFixtures(t) {
		if c.isError || c.tree == nil {
			continue
		}

		tree, err := Parse(c.source, optionsOf(c))
		if err != nil {
			t.Errorf("%s: %s", c.name, err.Message.String())
			failed++
			continue
		}

		want := c.tree
		if wrap, ok := wrappers[c.scope]; ok {
			if object, isObject := want.(*jsutil.Object); isObject {
				if typeValue, hasType := object.Get(jsutil.FromString("type")); hasType {
					if text, isText := typeValue.(jsutil.JSONString); isText {
						if !strings.EqualFold(jsutil.Str(text).String(), c.scope) {
							want = wrap(want)
						}
					}
				}
			}
		}

		got := jsutil.Stringify(withoutLoc(ast.ToPlain(tree)), 0).String()
		expected := jsutil.Stringify(withoutLoc(want), 0).String()

		if got != expected {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, expected)
			failed++
			continue
		}

		passed++
	}

	t.Logf("%d fixture trees matched, %d did not", passed, failed)
}

// TestFixtureErrors checks that every fixture marked as an error is reported as one.
func TestFixtureErrors(t *testing.T) {
	checked := 0

	for _, c := range loadFixtures(t) {
		if !c.isError {
			continue
		}

		// An error fixture states that the parse reports an error; the original's own test
		// loader hands the parser a handler that raises what it is given, so that a recovered
		// error ends the parse rather than becoming a Raw node.
		options := optionsOf(c)
		options.OnParseError = func(err *jsutil.Error, _ *ast.Node) {
			panic(err)
		}

		if _, err := Parse(c.source, options); err == nil {
			t.Errorf("%s: parsed without an error", c.name)
			continue
		}

		checked++
	}

	if checked == 0 {
		t.Fatal("no error fixtures were found")
	}

	t.Logf("%d error fixtures were reported", checked)
}

// TestUnknownContext checks the error an unknown parse context raises.
func TestUnknownContext(t *testing.T) {
	options := NewOptions()
	options.Context = "nope"

	_, err := Parse(jsutil.FromString("a{b:c}"), options)
	if err == nil {
		t.Fatal("an unknown context did not raise an error")
	}
	if got, want := err.Message.String(), "Unknown context `nope`"; got != want {
		t.Errorf("the message is %q, want %q", got, want)
	}
}

// TestPositions checks the places a parse records over the newline forms.
func TestPositions(t *testing.T) {
	options := NewOptions()
	options.Positions = true

	tree, err := Parse(jsutil.FromString("a\r\n{\n  b : c\n}"), options)
	if err != nil {
		t.Fatalf("parsing failed: %s", err.Message.String())
	}
	if tree.Loc == nil {
		t.Fatal("the tree carries no location")
	}
	if tree.Loc.Start.Line != 1 || tree.Loc.Start.Column != 1 {
		t.Errorf("the tree starts at %d:%d, want 1:1", tree.Loc.Start.Line, tree.Loc.Start.Column)
	}
	if tree.Loc.End.Line != 4 || tree.Loc.End.Column != 2 {
		t.Errorf("the tree ends at %d:%d, want 4:2", tree.Loc.End.Line, tree.Loc.End.Column)
	}
}
