package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
)

// corpusEntry is one tree of the recorded corpus, with the CSS the original writes for it in each
// of the two whitespace modes.
type corpusEntry struct {
	name string
	tree jsutil.Value
	safe jsutil.Str
	spec jsutil.Str
}

func loadCorpus(t *testing.T) []corpusEntry {
	t.Helper()

	file, err := os.Open("testdata/corpus.json")
	if err != nil {
		t.Fatalf("open corpus: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()

	value, err := decodeJSON(decoder)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}

	array, ok := value.(jsutil.Array)
	if !ok {
		t.Fatal("corpus is not an array")
	}

	entries := make([]corpusEntry, 0, len(array))
	for _, item := range array {
		object, ok := item.(*jsutil.Object)
		if !ok {
			t.Fatal("corpus entry is not an object")
		}
		entry := corpusEntry{}
		if name, ok := object.Get(jsutil.FromString("name")); ok {
			entry.name = jsutil.Str(name.(jsutil.JSONString)).String()
		}
		entry.tree, _ = object.Get(jsutil.FromString("ast"))
		if text, ok := object.Get(jsutil.FromString("safe")); ok {
			entry.safe = jsutil.Str(text.(jsutil.JSONString))
		}
		if text, ok := object.Get(jsutil.FromString("spec")); ok {
			entry.spec = jsutil.Str(text.(jsutil.JSONString))
		}
		entries = append(entries, entry)
	}

	return entries
}

// TestGenerateMatchesOriginal writes every tree of the recorded corpus in both modes and asks for
// the same bytes the original wrote.
func TestGenerateMatchesOriginal(t *testing.T) {
	entries := loadCorpus(t)
	if len(entries) == 0 {
		t.Fatal("empty corpus")
	}

	for _, entry := range entries {
		node := ast.FromPlain(entry.tree)
		if node == nil {
			t.Fatalf("%s: tree is not a node", entry.name)
		}

		for _, mode := range []struct {
			name  string
			value string
			want  jsutil.Str
		}{
			{"safe", "", entry.safe},
			{"spec", "spec", entry.spec},
		} {
			got, err := Generate(node, mode.value)
			if err != nil {
				t.Errorf("%s [%s]: %v", entry.name, mode.name, err)
				continue
			}
			if !got.Equal(mode.want) {
				t.Errorf("%s [%s]:\n got %q\nwant %q", entry.name, mode.name, got.String(), mode.want.String())
			}
		}
	}
}

// TestGenerateSafeModeNames checks that the mode name is read the way the original reads it: only
// "spec" picks the spec table and anything else falls back to the default one.
func TestGenerateSafeModeNames(t *testing.T) {
	// "a" followed by "#id" needs a whitespace in safe mode but not in the spec one.
	selector := ast.New("Selector").Set("children", ast.NewList().FromSlice([]*ast.Node{
		ast.New("TypeSelector").SetString("name", "a"),
		ast.New("IdSelector").SetString("name", "id"),
	}))

	for _, test := range []struct{ mode, want string }{
		{"", "a#id"},
		{"safe", "a#id"},
		{"spec", "a#id"},
		{"nonsense", "a#id"},
	} {
		got, err := Generate(selector, test.mode)
		if err != nil {
			t.Fatalf("mode %q: %v", test.mode, err)
		}
		if got.String() != test.want {
			t.Errorf("mode %q: got %q, want %q", test.mode, got.String(), test.want)
		}
	}
}

// TestGenerateModesDiffer checks a tree the two tables disagree about: the safe table keeps a hash
// token away from the identifier before it, the spec table does not.
func TestGenerateModesDiffer(t *testing.T) {
	value := ast.New("Value").Set("children", ast.NewList().FromSlice([]*ast.Node{
		ast.New("Identifier").SetString("name", "solid"),
		ast.New("Hash").SetString("value", "000"),
	}))

	safe, err := Generate(value, "")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := Generate(value, "spec")
	if err != nil {
		t.Fatal(err)
	}

	if safe.String() != "solid #000" {
		t.Errorf("safe: got %q, want %q", safe.String(), "solid #000")
	}
	if spec.String() != "solid#000" {
		t.Errorf("spec: got %q, want %q", spec.String(), "solid#000")
	}
}

// TestGenerateUnknownNodeType asks for the error the original throws for a type it has no
// generator for.
func TestGenerateUnknownNodeType(t *testing.T) {
	_, err := Generate(ast.New("Whatever"), "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if err.Name != "Error" {
		t.Errorf("name: got %q, want %q", err.Name, "Error")
	}
	if got := err.Message.String(); got != "Unknown node type: Whatever" {
		t.Errorf("message: got %q", got)
	}
}

// TestGenerateRatioWithoutRight keeps the defect the original has: a ratio with no right side is
// written by handing the node handler a number, so the type it reads is undefined.
func TestGenerateRatioWithoutRight(t *testing.T) {
	ratio := ast.New("Ratio").
		Set("left", ast.New("Number").SetString("value", "16")).
		Set("right", nil)

	_, err := Generate(ratio, "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Message.String(); got != "Unknown node type: undefined" {
		t.Errorf("message: got %q", got)
	}
}

// TestGenerateNumberField writes a field that holds a number rather than a string, which a tree
// read from JSON can carry.
func TestGenerateNumberField(t *testing.T) {
	number := ast.New("Number").Set("value", jsutil.Num(1.5))

	got, err := Generate(number, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "1.5" {
		t.Errorf("got %q, want %q", got.String(), "1.5")
	}
}

// TestGenerateDimensionAddsNumbers keeps the JavaScript addition of two number fields, which the
// original's `node.value + node.unit` performs.
func TestGenerateDimensionAddsNumbers(t *testing.T) {
	dimension := ast.New("Dimension").Set("value", jsutil.Int(1)).Set("unit", jsutil.Int(2))

	got, err := Generate(dimension, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "3" {
		t.Errorf("got %q, want %q", got.String(), "3")
	}
}

// TestGenerateImportantFlag writes both shapes of the important field.
func TestGenerateImportantFlag(t *testing.T) {
	for _, test := range []struct {
		important any
		want      string
	}{
		{true, "color:red!important"},
		{jsutil.FromString("ie"), "color:red!ie"},
		{false, "color:red"},
	} {
		declaration := ast.New("Declaration").
			Set("important", test.important).
			SetString("property", "color").
			Set("value", ast.New("Value").Set("children", ast.NewList().FromSlice([]*ast.Node{
				ast.New("Identifier").SetString("name", "red"),
			})))

		got, err := Generate(declaration, "")
		if err != nil {
			t.Fatal(err)
		}
		if got.String() != test.want {
			t.Errorf("got %q, want %q", got.String(), test.want)
		}
	}
}

// TestGenerateBackslashDelim keeps the newline the original writes after a delim that is a
// backslash.
func TestGenerateBackslashDelim(t *testing.T) {
	g := &generator{buffer: jsutil.Str{}, tokenBefore: safeTokenBefore}
	g.token(9, jsutil.FromString("\\"), false) // 9 is the delim token type

	if got := g.buffer.String(); got != "\\\n" {
		t.Errorf("got %q, want %q", got, "\\\n")
	}
}

// TestEncodeString serializes strings the way § 2.1 of CSSOM asks for.
func TestEncodeString(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"", `""`},
		{"abc", `"abc"`},
		{"a\"b", `"a\"b"`},
		{"a\\b", `"a\\b"`},
		{"a\x00b", "\"a�b\""},
		{"\x01a", `"\1 a"`},
		{"\x01z", `"\1z"`},
		{"\x7f", `"\7f"`},
	} {
		if got := encodeString(jsutil.FromString(test.in)).String(); got != test.want {
			t.Errorf("encodeString(%q): got %q, want %q", test.in, got, test.want)
		}
	}
}

// TestEncodeURL escapes what would otherwise end a url token early.
func TestEncodeURL(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"", "url()"},
		{"a.png", "url(a.png)"},
		{"a b", `url(a\ b)`},
		{"a(b)", `url(a\(b\))`},
		{"'\"", `url(\'\")`},
		{"\x01a", `url(\1 a)`},
	} {
		if got := encodeURL(jsutil.FromString(test.in)).String(); got != test.want {
			t.Errorf("encodeURL(%q): got %q, want %q", test.in, got, test.want)
		}
	}
}

// decodeJSON reads one JSON value, keeping the order the keys of an object were written in.
func decodeJSON(decoder *json.Decoder) (jsutil.Value, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	return decodeJSONFrom(decoder, token)
}

func decodeJSONFrom(decoder *json.Decoder, token json.Token) (jsutil.Value, error) {
	switch value := token.(type) {
	case nil:
		return jsutil.Null{}, nil
	case bool:
		return jsutil.Bool(value), nil
	case string:
		return jsutil.JSONString(jsutil.FromString(value)), nil
	case json.Number:
		if whole, err := strconv.Atoi(value.String()); err == nil {
			return jsutil.Int(whole), nil
		}
		number, err := value.Float64()
		if err != nil {
			return nil, err
		}
		return jsutil.Num(number), nil
	case json.Delim:
		switch value {
		case '[':
			array := jsutil.Array{}
			for decoder.More() {
				item, err := decodeJSON(decoder)
				if err != nil {
					return nil, err
				}
				array = append(array, item)
			}
			if _, err := decoder.Token(); err != nil {
				return nil, err
			}
			return array, nil
		case '{':
			object := jsutil.NewObject()
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok {
					return nil, fmt.Errorf("object key is not a string: %v", key)
				}
				item, err := decodeJSON(decoder)
				if err != nil {
					return nil, err
				}
				object.Set(jsutil.FromString(name), item)
			}
			if _, err := decoder.Token(); err != nil {
				return nil, err
			}
			return object, nil
		}
	}
	return nil, fmt.Errorf("unexpected JSON token: %v", token)
}
