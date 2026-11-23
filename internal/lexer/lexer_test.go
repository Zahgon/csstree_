package lexer

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/csstree/csstree/internal/dict"
	"github.com/csstree/csstree/internal/jsutil"
)

func testLexer(t *testing.T) *Lexer {
	t.Helper()

	read := func(name string) jsutil.Value {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		value, err := jsutil.ParseJSONBytes(raw)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		return value
	}

	d, err := dict.Build(
		read("../../data/mdn-data/at-rules.json"),
		read("../../data/mdn-data/properties.json"),
		read("../../data/mdn-data/syntaxes.json"),
		read("../../data/patch.json"),
	)
	if err != nil {
		t.Fatalf("resolving the dictionary: %v", err)
	}

	return New(d)
}

// matchFixtures are the repository's own definition matching fixtures: a definition, the values
// that match it and the values that do not.
func matchFixtures(t *testing.T, dir string, syntaxFromKey bool) map[string]*jsutil.Object {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
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
			if fields, ok := entry.(*jsutil.Object); ok {
				out[file+":"+key.String()] = fields
			}
		}
	}

	if len(out) == 0 {
		t.Fatalf("no fixtures were found under %s", dir)
	}

	return out
}

func stringsOf(object *jsutil.Object, key string) ([]jsutil.Str, bool) {
	value, ok := object.Get(jsutil.FromString(key))
	if !ok {
		return nil, false
	}
	list, ok := value.(jsutil.Array)
	if !ok {
		return nil, false
	}
	out := []jsutil.Str{}
	for _, item := range list {
		if text, ok := item.(jsutil.JSONString); ok {
			out = append(out, jsutil.Str(text))
		}
	}
	return out, true
}

// TestDefinitionSyntaxMatch matches the values of every definition matching fixture against the
// definition the fixture is keyed by.
func TestDefinitionSyntaxMatch(t *testing.T) {
	l := testLexer(t)
	checked := 0

	skipped := 0

	for name, fields := range matchFixtures(t, "../../fixtures/definition-syntax-match", true) {
		// The key after the file name is the definition itself, unless the fixture states one.
		definition := jsutil.FromString(name[indexAfterColon(name):])
		if value, ok := fields.Get(jsutil.FromString("syntax")); ok {
			if text, isText := value.(jsutil.JSONString); isText {
				definition = jsutil.Str(text)
			}
		}

		// A fixture that brings types or properties of its own is matched against a dictionary
		// built for it, which this test does not build; the differential corpus covers those.
		if _, ok := fields.Get(jsutil.FromString("types")); ok {
			skipped++
			continue
		}
		if _, ok := fields.Get(jsutil.FromString("properties")); ok {
			skipped++
			continue
		}

		if valid, ok := stringsOf(fields, "valid"); ok {
			for i, value := range valid {
				result := l.Match(definition, TextValue(value))
				if result.Matched == nil {
					t.Errorf("%s: valid[%d] %q did not match", name, i, value.String())
				}
				checked++
			}
		}

		if invalid, ok := stringsOf(fields, "invalid"); ok {
			for i, value := range invalid {
				result := l.Match(definition, TextValue(value))
				if result.Matched != nil {
					t.Errorf("%s: invalid[%d] %q matched", name, i, value.String())
				}
				checked++
			}
		}
	}

	t.Log("checked " + strconv.Itoa(checked) + " values, skipped " + strconv.Itoa(skipped) +
		" definitions that bring a dictionary of their own")
}

// indexAfterColon is where the definition begins in a fixture name, which is the file name, a
// colon and then the definition.
func indexAfterColon(name string) int {
	for i := 0; i < len(name); i++ {
		if name[i] == ':' {
			return i + 1
		}
	}
	return 0
}

// TestPropertyMatching checks a few values against the property dictionary.
func TestPropertyMatching(t *testing.T) {
	l := testLexer(t)

	matching := [][2]string{
		{"color", "red"},
		{"color", "#ff0000"},
		{"color", "rgb(1 2 3)"},
		{"color", "inherit"},
		{"margin", "1px 2px 3px 4px"},
		{"display", "inline-flex"},
		{"width", "calc(100% - 1px)"},
		{"background", "url(x) red"},
	}

	for _, pair := range matching {
		result := l.MatchProperty(jsutil.FromString(pair[0]), TextValue(jsutil.FromString(pair[1])))
		if result.Matched == nil {
			message := ""
			if result.Error != nil {
				message = result.Error.Message.String()
			}
			t.Errorf("%s: %q did not match: %s", pair[0], pair[1], message)
		}
	}

	notMatching := [][2]string{
		{"color", "1px"},
		{"display", "nope"},
		{"margin", "red"},
	}

	for _, pair := range notMatching {
		result := l.MatchProperty(jsutil.FromString(pair[0]), TextValue(jsutil.FromString(pair[1])))
		if result.Matched != nil {
			t.Errorf("%s: %q matched and should not have", pair[0], pair[1])
		}
		if result.Error == nil || result.Error.Name != "SyntaxMatchError" {
			t.Errorf("%s: %q did not report a match error", pair[0], pair[1])
		}
	}
}

// TestUnknownNames checks what the lexer reports for names it does not know.
func TestUnknownNames(t *testing.T) {
	l := testLexer(t)

	if err := l.CheckPropertyName(jsutil.FromString("colour")); err == nil {
		t.Error("an unknown property was accepted")
	} else if got, want := err.Message.String(), "Unknown property `colour`"; got != want {
		t.Errorf("the message is %q, want %q", got, want)
	}

	if err := l.CheckAtruleName(jsutil.FromString("nope")); err == nil {
		t.Error("an unknown at-rule was accepted")
	} else if got, want := err.Message.String(), "Unknown at-rule `@nope`"; got != want {
		t.Errorf("the message is %q, want %q", got, want)
	}

	if err := l.CheckPropertyName(jsutil.FromString("color")); err != nil {
		t.Errorf("a known property was refused: %s", err.Message.String())
	}
}

// TestValidate checks that every definition of the dictionary resolves.
func TestValidate(t *testing.T) {
	l := testLexer(t)

	if _, isNull := l.Validate().(jsutil.Null); !isNull {
		t.Error("the dictionary has definitions that do not resolve")
	}
}
