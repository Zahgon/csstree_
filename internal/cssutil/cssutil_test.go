package cssutil

import (
	"testing"

	"github.com/csstree/csstree/internal/jsutil"
)

func str(s string) jsutil.Str { return jsutil.FromString(s) }

// TestDecodeString checks the text a quoted string stands for, including the escapes.
func TestDecodeString(t *testing.T) {
	cases := [][2]string{
		{`"a"`, "a"},
		{`'a'`, "a"},
		{`"a`, "a"},
		{`""`, ""},
		{`"a\62 c"`, "abc"},
		{`"a\"b"`, `a"b`},
		{"\"a\\\nb\"", "ab"},
		{`"\0"`, "�"},
		{`"\110000"`, "�"},
		{`"\d800"`, "�"},
		{`"a\\b"`, `a\b`},
	}

	for _, c := range cases {
		if got := DecodeString(str(c[0])).String(); got != c[1] {
			t.Errorf("DecodeString(%q) is %q, want %q", c[0], got, c[1])
		}
	}
}

// TestEncodeString checks the way a string is written back out.
func TestEncodeString(t *testing.T) {
	cases := [][2]string{
		{"a", `"a"`},
		{`a"b`, `"a\"b"`},
		{`a\b`, `"a\\b"`},
		{"\u0000", "\"�\""},
		{"\u0001", `"\1"`},
		{"\u0001a", `"\1 a"`},
		{"\u0001z", `"\1z"`},
		{"\u007f", `"\7f"`},
	}

	for _, c := range cases {
		if got := EncodeString(str(c[0]), false).String(); got != c[1] {
			t.Errorf("EncodeString(%q) is %q, want %q", c[0], got, c[1])
		}
	}

	if got := EncodeString(str("a'b"), true).String(); got != `'a\'b'` {
		t.Errorf("EncodeString with an apostrophe is %q", got)
	}
}

// TestIdent checks identifier decoding and encoding.
func TestIdent(t *testing.T) {
	if got := DecodeIdent(str(`a\62 c`)).String(); got != "abc" {
		t.Errorf("DecodeIdent is %q, want %q", got, "abc")
	}

	cases := [][2]string{
		{"-", `\-`},
		{"a", "a"},
		{"1a", `\31 a`},
		{"-1a", `-\31 a`},
		{"a b", `a\ b`},
		{"\u0000", "�"},
		{"--a", "--a"},
	}

	for _, c := range cases {
		if got := EncodeIdent(str(c[0])).String(); got != c[1] {
			t.Errorf("EncodeIdent(%q) is %q, want %q", c[0], got, c[1])
		}
	}
}

// TestURL checks url decoding and encoding.
func TestURL(t *testing.T) {
	cases := [][2]string{
		{"url(a)", "a"},
		{"url( a )", "a"},
		{"url(a", "a"},
		{"url()", ""},
		{`url(a\62 c)`, "abc"},
	}

	for _, c := range cases {
		if got := DecodeURL(str(c[0])).String(); got != c[1] {
			t.Errorf("DecodeURL(%q) is %q, want %q", c[0], got, c[1])
		}
	}

	if got := EncodeURL(str("a b")).String(); got != `url(a\ b)` {
		t.Errorf("EncodeURL is %q", got)
	}
	if got := EncodeURL(str("a(b")).String(); got != `url(a\(b)` {
		t.Errorf("EncodeURL is %q", got)
	}
}

// TestNames checks the descriptors a keyword and a property name are read into.
func TestNames(t *testing.T) {
	keyword := Keyword(str("-WEBKIT-Foo"))
	if keyword.Name.String() != "-webkit-foo" || keyword.Vendor.String() != "-webkit-" ||
		keyword.Basename.String() != "foo" || keyword.Custom {
		t.Errorf("the keyword descriptor is %+v", keyword)
	}

	property := Property(str("*-moz-Box"))
	if property.Hack.String() != "*" || property.Name.String() != "-moz-box" ||
		property.Vendor.String() != "-moz-" || property.Basename.String() != "box" ||
		property.Prefix.String() != "*-moz-" || property.Custom {
		t.Errorf("the property descriptor is %+v", property)
	}

	custom := Property(str("--Custom"))
	if !custom.Custom || custom.Name.String() != "--Custom" {
		t.Errorf("the custom property descriptor is %+v", custom)
	}

	if !IsCustomProperty(str("--a"), 0) || IsCustomProperty(str("-a"), 0) {
		t.Error("the custom property test is wrong")
	}

	if got := VendorPrefix(str("-webkit-foo"), 0).String(); got != "-webkit-" {
		t.Errorf("VendorPrefix is %q", got)
	}
	if got := VendorPrefix(str("foo"), 0).String(); got != "" {
		t.Errorf("VendorPrefix of a plain name is %q", got)
	}
}
