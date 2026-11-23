package syntaxdef

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/csstree/csstree/internal/jsutil"
)

func str(s string) jsutil.Str { return jsutil.FromString(s) }

// fixtureEntries are the repository's own definition syntax fixtures: a definition and, for the
// ones that cannot be read, the error it raises.
func fixtureEntries(t *testing.T) map[string]*jsutil.Object {
	t.Helper()

	files, err := filepath.Glob("../../fixtures/definition-syntax/*.json")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)

	out := map[string]*jsutil.Object{}

	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		value, err := jsutil.ParseJSONBytes(raw)
		if err != nil {
			t.Fatal(err)
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
				if fields, ok := item.(*jsutil.Object); ok {
					out[file+":"+key.String()+"#"+strconv.Itoa(i)] = fields
				}
			}
		}
	}

	if len(out) == 0 {
		t.Fatal("no fixtures were found")
	}

	return out
}

func textField(object *jsutil.Object, key string) (jsutil.Str, bool) {
	value, ok := object.Get(jsutil.FromString(key))
	if !ok {
		return nil, false
	}
	text, ok := value.(jsutil.JSONString)
	if !ok {
		return nil, false
	}
	return jsutil.Str(text), true
}

// TestFixtureRoundTrip reads every fixture definition and writes it out again, which must give
// the definition back unchanged once it has been read once.
func TestFixtureRoundTrip(t *testing.T) {
	checked, skipped := 0, 0

	for name, fields := range fixtureEntries(t) {
		definition, ok := textField(fields, "syntax")
		if !ok {
			skipped++
			continue
		}

		tree, err := Parse(definition)
		if err != nil {
			if _, isError := fields.Get(jsutil.FromString("error")); isError {
				checked++
				continue
			}
			t.Errorf("%s: %s", name, err.Message.String())
			continue
		}

		if _, isError := fields.Get(jsutil.FromString("error")); isError {
			t.Errorf("%s: %q was read without an error", name, definition.String())
			continue
		}

		written := Generate(tree, false, false)

		again, err := Parse(written)
		if err != nil {
			t.Errorf("%s: writing %q gave %q, which cannot be read back: %s",
				name, definition.String(), written.String(), err.Message.String())
			continue
		}

		if got, want := Generate(again, false, false).String(), written.String(); got != want {
			t.Errorf("%s: writing %q twice gives %q then %q", name, definition.String(), want, got)
		}

		checked++
	}

	t.Logf("%d definitions checked, %d without a definition of their own", checked, skipped)
}

// TestParseShape checks the tree a definition is read into.
func TestParseShape(t *testing.T) {
	tree, err := Parse(str("<length> | auto"))
	if err != nil {
		t.Fatalf("reading failed: %s", err.Message.String())
	}

	if tree.Type != "Group" {
		t.Fatalf("the tree is a %s, want a Group", tree.Type)
	}
	if got := tree.Str("combinator").String(); got != "|" {
		t.Errorf("the combinator is %q, want %q", got, "|")
	}

	terms := tree.Terms("terms")
	if len(terms) != 2 {
		t.Fatalf("the group holds %d terms, want 2", len(terms))
	}
	if terms[0].Type != "Type" || terms[0].Str("name").String() != "length" {
		t.Errorf("the first term is %s %q", terms[0].Type, terms[0].Str("name").String())
	}
	if terms[1].Type != "Keyword" || terms[1].Str("name").String() != "auto" {
		t.Errorf("the second term is %s %q", terms[1].Type, terms[1].Str("name").String())
	}
}

// TestGenerateForms checks the three forms a definition is written in.
func TestGenerateForms(t *testing.T) {
	tree, err := Parse(str("[ a | b ]{1,4}"))
	if err != nil {
		t.Fatalf("reading failed: %s", err.Message.String())
	}

	if got, want := Generate(tree, false, false).String(), "[ a | b ]{1,4}"; got != want {
		t.Errorf("the default form is %q, want %q", got, want)
	}
	if got, want := Generate(tree, false, true).String(), "[a|b]{1,4}"; got != want {
		t.Errorf("the compact form is %q, want %q", got, want)
	}
}

// TestWalkOrder checks the order a definition is walked in.
func TestWalkOrder(t *testing.T) {
	tree, err := Parse(str("a | b"))
	if err != nil {
		t.Fatalf("reading failed: %s", err.Message.String())
	}

	var order []string
	Walk(tree, func(node *Node) {
		order = append(order, "enter "+node.Type)
	}, func(node *Node) {
		order = append(order, "leave "+node.Type)
	})

	want := []string{"enter Group", "enter Keyword", "leave Keyword", "enter Keyword", "leave Keyword", "leave Group"}
	if len(order) != len(want) {
		t.Fatalf("the walk is %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("the walk is %v, want %v", order, want)
		}
	}
}

// TestBrokenDefinitions checks that definitions that cannot be read are reported.
func TestBrokenDefinitions(t *testing.T) {
	for _, definition := range []string{"<", "<>", "[", "a{", "a??", "|", "a&&&", "<length [,]>"} {
		if _, err := Parse(str(definition)); err == nil {
			t.Errorf("%q was read without an error", definition)
		}
	}

	// The original reads an empty definition as an empty group, and does not check that the
	// bounds of a multiplier are the right way round; both are kept.
	for _, definition := range []string{"", "a{2,1}"} {
		if _, err := Parse(str(definition)); err != nil {
			t.Errorf("%q was refused: %s", definition, err.Message.String())
		}
	}
}
