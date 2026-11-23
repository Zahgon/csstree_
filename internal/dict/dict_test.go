package dict

import (
	"os"
	"testing"

	"github.com/csstree/csstree/internal/jsutil"
)

func readData(t *testing.T, path string) jsutil.Value {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	value, err := jsutil.ParseJSONBytes(raw)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return value
}

func testDict(t *testing.T) *Dict {
	t.Helper()

	d, err := Build(
		readData(t, "../../data/mdn-data/at-rules.json"),
		readData(t, "../../data/mdn-data/properties.json"),
		readData(t, "../../data/mdn-data/syntaxes.json"),
		readData(t, "../../data/patch.json"),
	)
	if err != nil {
		t.Fatalf("resolving the dictionary: %v", err)
	}
	return d
}

func definition(t *testing.T, object *jsutil.Object, name string) string {
	t.Helper()

	value, ok := object.Get(jsutil.FromString(name))
	if !ok {
		t.Fatalf("%s is not in the dictionary", name)
	}
	text, ok := value.(jsutil.JSONString)
	if !ok {
		t.Fatalf("the definition of %s is not text", name)
	}
	return jsutil.Str(text).String()
}

// TestResolvedDictionary checks the shape of the dictionary the data and the patch resolve to.
func TestResolvedDictionary(t *testing.T) {
	d := testDict(t)

	if d.Types.Len() < 100 {
		t.Errorf("the dictionary holds %d types, which is too few", d.Types.Len())
	}
	if d.Properties.Len() < 300 {
		t.Errorf("the dictionary holds %d properties, which is too few", d.Properties.Len())
	}
	if len(d.Atrules.Keys()) < 10 {
		t.Errorf("the dictionary holds %d at-rules, which is too few", len(d.Atrules.Keys()))
	}

	if got, want := definition(t, d.Properties, "color"), "<color>"; got != want {
		t.Errorf("the definition of color is %q, want %q", got, want)
	}
}

// TestPreludeFromDefinition checks the prelude read out of an at-rule's full definition: a rule
// whose definition is only a block has none.
func TestPreludeFromDefinition(t *testing.T) {
	d := testDict(t)

	media, ok := d.Atrules.Get(jsutil.FromString("media"))
	if !ok {
		t.Fatal("@media is not in the dictionary")
	}
	if !media.HasPrelude {
		t.Error("@media has no prelude")
	}

	fontFace, ok := d.Atrules.Get(jsutil.FromString("font-face"))
	if !ok {
		t.Fatal("@font-face is not in the dictionary")
	}
	if fontFace.HasPrelude {
		t.Errorf("@font-face has the prelude %q and should have none", fontFace.Prelude.String())
	}
	if fontFace.Descriptors == nil || fontFace.Descriptors.Len() == 0 {
		t.Error("@font-face has no descriptors")
	}
}

// TestPatchApplied checks that the repository's patch reaches the resolved dictionary: it adds
// an at-rule prelude the data does not carry, and replaces a descriptor definition.
func TestPatchApplied(t *testing.T) {
	d := testDict(t)

	charset, ok := d.Atrules.Get(jsutil.FromString("charset"))
	if !ok {
		t.Fatal("@charset is not in the dictionary")
	}
	if got, want := charset.Prelude.String(), "<string>"; got != want {
		t.Errorf("the prelude of @charset is %q, want %q", got, want)
	}

	fontFace, _ := d.Atrules.Get(jsutil.FromString("font-face"))
	if got, want := definition(t, fontFace.Descriptors, "unicode-range"), "<urange>#"; got != want {
		t.Errorf("the unicode-range descriptor of @font-face is %q, want %q", got, want)
	}
}

// TestOrderIsKept checks that the dictionary keeps the order its entries were read in, which is
// the order it is written out in.
func TestOrderIsKept(t *testing.T) {
	d := testDict(t)

	keys := d.Types.Keys()
	if len(keys) < 2 {
		t.Fatal("the dictionary holds too few types")
	}

	raw := readData(t, "../../data/mdn-data/syntaxes.json").(*jsutil.Object)
	first := raw.Keys()[0].String()

	if got := keys[0].String(); got != first {
		t.Errorf("the first type is %q, want %q, the first of the data", got, first)
	}
}
