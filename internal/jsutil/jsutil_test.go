package jsutil

import (
	"math"
	"testing"
)

// TestStrEncoding checks that a string survives being read and written as UTF-8, and that a code
// unit that is not part of a pair is written as the replacement character, as a runtime writes it.
func TestStrEncoding(t *testing.T) {
	for _, text := range []string{"", "a", "héllo", "\U0001F600", "a\u0000b"} {
		if got := FromString(text).String(); got != text {
			t.Errorf("%q became %q", text, got)
		}
	}

	lone := Str{0xD800, 'a'}
	if got, want := lone.String(), "�a"; got != want {
		t.Errorf("a lone surrogate became %q, want %q", got, want)
	}

	pair := Str{0xD83D, 0xDE00}
	if got, want := pair.String(), "\U0001F600"; got != want {
		t.Errorf("a surrogate pair became %q, want %q", got, want)
	}

	// A byte that is not well formed UTF-8 is read as one replacement character.
	if got, want := FromUTF8([]byte{0xFF, 'a'}).String(), "�a"; got != want {
		t.Errorf("a bad byte became %q, want %q", got, want)
	}
}

// TestStrOperations checks the string operations the CSS code is written against.
func TestStrOperations(t *testing.T) {
	s := FromString("abcdef")

	if got := s.CharCodeAt(0); got != 'a' {
		t.Errorf("CharCodeAt(0) is %d", got)
	}
	if got := s.CharCodeAt(100); got != 0 {
		t.Errorf("CharCodeAt past the end is %d, want 0", got)
	}
	if got := s.Substring(1, 3).String(); got != "bc" {
		t.Errorf("Substring is %q", got)
	}
	if got := s.Substring(3, 1).String(); got != "bc" {
		t.Errorf("Substring with its ends the wrong way round is %q", got)
	}
	if got := s.Slice(4).String(); got != "ef" {
		t.Errorf("Slice is %q", got)
	}
	if got := s.IndexOfFrom(FromString("cd"), 0); got != 2 {
		t.Errorf("IndexOfFrom is %d", got)
	}
	if got := s.IndexOfFrom(FromString("cd"), 3); got != -1 {
		t.Errorf("IndexOfFrom past the match is %d", got)
	}
	if got := FromString("ABC").ToLower().String(); got != "abc" {
		t.Errorf("ToLower is %q", got)
	}
}

// TestNumberToString checks the text JavaScript prints a number as.
func TestNumberToString(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{1.5, "1.5"},
		{0.1, "0.1"},
		{1e21, "1e+21"},
		{1e20, "100000000000000000000"},
		{1e-7, "1e-7"},
		{1e-6, "0.000001"},
		{math.Inf(1), "Infinity"},
		{math.Inf(-1), "-Infinity"},
		{math.NaN(), "NaN"},
	}

	for _, c := range cases {
		if got := NumberToString(c.value); got != c.want {
			t.Errorf("NumberToString(%v) is %q, want %q", c.value, got, c.want)
		}
	}
}

// TestStringify checks the JSON text an indented value is written as.
func TestStringify(t *testing.T) {
	object := NewObject()
	object.SetString("b", Int(1))
	object.SetString("a", JSONString(FromString("x\"y\n")))
	object.SetString("c", Array{Bool(true), Null{}})

	if got, want := Stringify(object, 0).String(),
		`{"b":1,"a":"x\"y\n","c":[true,null]}`; got != want {
		t.Errorf("the compact form is %s, want %s", got, want)
	}

	if got, want := Stringify(object, 2).String(),
		"{\n  \"b\": 1,\n  \"a\": \"x\\\"y\\n\",\n  \"c\": [\n    true,\n    null\n  ]\n}"; got != want {
		t.Errorf("the indented form is %q, want %q", got, want)
	}

	// An empty object and an empty array are written without any whitespace inside.
	empty := NewObject()
	empty.SetString("o", NewObject())
	empty.SetString("a", Array{})
	if got, want := Stringify(empty, 2).String(), "{\n  \"o\": {},\n  \"a\": []\n}"; got != want {
		t.Errorf("the empty forms are %q, want %q", got, want)
	}

	// A code unit that is not part of a pair is escaped, as well formed JSON asks.
	if got, want := Stringify(JSONString(Str{0xD800}), 0).String(), `"\ud800"`; got != want {
		t.Errorf("a lone surrogate is written as %s, want %s", got, want)
	}
}

// TestParseJSON checks that reading JSON keeps the order the keys were written in.
func TestParseJSON(t *testing.T) {
	value, err := ParseJSON(FromString(`{"b":1,"a":{"c":[1,2.5,"x",true,null]}}`))
	if err != nil {
		t.Fatalf("reading failed: %v", err)
	}

	if got, want := Stringify(value, 0).String(), `{"b":1,"a":{"c":[1,2.5,"x",true,null]}}`; got != want {
		t.Errorf("a round trip gives %s, want %s", got, want)
	}

	if _, err := ParseJSON(FromString(`{"a":}`)); err == nil {
		t.Error("bad JSON was read without an error")
	}
}

// TestParseFloat checks the number JavaScript reads out of the front of a string.
func TestParseFloat(t *testing.T) {
	cases := []struct {
		text string
		want float64
	}{
		{"1", 1},
		{"1.5px", 1.5},
		{"  -2e3  ", -2000},
		{".5", 0.5},
		{"+1", 1},
	}

	for _, c := range cases {
		if got := ParseFloat(FromString(c.text)); got != c.want {
			t.Errorf("ParseFloat(%q) is %v, want %v", c.text, got, c.want)
		}
	}

	if got := ParseFloat(FromString("x")); !math.IsNaN(got) {
		t.Errorf("ParseFloat of text that is not a number is %v", got)
	}
}
