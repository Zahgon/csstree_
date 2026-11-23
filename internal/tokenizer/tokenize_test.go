package tokenizer

import (
	"testing"

	"github.com/csstree/csstree/internal/jsutil"
)

type tok struct {
	name  string
	chunk string
}

func collect(t *testing.T, css string) []tok {
	t.Helper()

	source := jsutil.FromString(css)
	var got []tok
	prevEnd := -1

	Tokenize(source, func(tokenType, start, end int) {
		if prevEnd >= 0 && start != prevEnd {
			t.Fatalf("token at %d does not start where the previous one ended (%d)", start, prevEnd)
		}
		if end <= start {
			t.Fatalf("token %d..%d is empty or reversed", start, end)
		}
		prevEnd = end
		got = append(got, tok{Names[tokenType], source.Substring(start, end).String()})
	})

	return got
}

// TestTokenizeTypes walks the token types, the escapes, the bad strings and the bad urls.
func TestTokenizeTypes(t *testing.T) {
	tests := []struct {
		css      string
		expected []tok
	}{
		{"a", []tok{{"ident-token", "a"}}},
		{"A-B_c", []tok{{"ident-token", "A-B_c"}}},
		{"--custom", []tok{{"ident-token", "--custom"}}},
		{"-a", []tok{{"ident-token", "-a"}}},
		{"\\61 b", []tok{{"ident-token", "\\61 b"}}},
		{"\\-a", []tok{{"ident-token", "\\-a"}}},
		{"a\\ b", []tok{{"ident-token", "a\\ b"}}},
		{"caf\u00e9", []tok{{"ident-token", "café"}}},
		{"calc(", []tok{{"function-token", "calc("}}},
		{"url(foo)", []tok{{"url-token", "url(foo)"}}},
		{"url( foo )", []tok{{"url-token", "url( foo )"}}},
		{"URL(\"x\")", []tok{{"function-token", "URL("}, {"string-token", "\"x\""}, {")-token", ")"}}},
		{"url(  )", []tok{{"url-token", "url(  )"}}},
		{"url(\\))", []tok{{"url-token", "url(\\))"}}},
		{"@media", []tok{{"at-keyword-token", "@media"}}},
		{"@-moz-x", []tok{{"at-keyword-token", "@-moz-x"}}},
		{"@\\41", []tok{{"at-keyword-token", "@\\41"}}},
		{"@1", []tok{{"delim-token", "@"}, {"number-token", "1"}}},
		{"#id", []tok{{"hash-token", "#id"}}},
		{"#1", []tok{{"hash-token", "#1"}}},
		{"#\\41", []tok{{"hash-token", "#\\41"}}},
		{"#", []tok{{"delim-token", "#"}}},
		{"\"abc\"", []tok{{"string-token", "\"abc\""}}},
		{"'a\\'b'", []tok{{"string-token", "'a\\'b'"}}},
		{"\"a\\\nb\"", []tok{{"string-token", "\"a\\\nb\""}}},
		{"\"a\nb\"", []tok{{"bad-string-token", "\"a\n"}, {"ident-token", "b"}, {"string-token", "\""}}},
		{"\"abc", []tok{{"string-token", "\"abc"}}},
		{"\"a\\", []tok{{"string-token", "\"a\\"}}},
		{"url(a\"b)", []tok{{"bad-url-token", "url(a\"b)"}}},
		{"url(a\u0007b)", []tok{{"bad-url-token", "url(a\u0007b)"}}},
		{"url(a b c)", []tok{{"bad-url-token", "url(a b c)"}}},
		{"url(a\\", []tok{{"bad-url-token", "url(a\\"}}},
		{"+1", []tok{{"number-token", "+1"}}},
		{"-1.5e-3", []tok{{"number-token", "-1.5e-3"}}},
		{".5", []tok{{"number-token", ".5"}}},
		{"1e3", []tok{{"number-token", "1e3"}}},
		{"12px", []tok{{"dimension-token", "12px"}}},
		{"50%", []tok{{"percentage-token", "50%"}}},
		{"1e", []tok{{"dimension-token", "1e"}}},
		{"+a", []tok{{"delim-token", "+"}, {"ident-token", "a"}}},
		{"-a", []tok{{"ident-token", "-a"}}},
		{".a", []tok{{"delim-token", "."}, {"ident-token", "a"}}},
		{" \t\n\r\f", []tok{{"whitespace-token", " \t\n\r\f"}}},
		{"<!--", []tok{{"CDO-token", "<!--"}}},
		{"-->", []tok{{"CDC-token", "-->"}}},
		{"<!-", []tok{{"delim-token", "<"}, {"delim-token", "!"}, {"delim-token", "-"}}},
		{":", []tok{{"colon-token", ":"}}},
		{";", []tok{{"semicolon-token", ";"}}},
		{",", []tok{{"comma-token", ","}}},
		{"[", []tok{{"[-token", "["}}},
		{"]", []tok{{"]-token", "]"}}},
		{"(", []tok{{"(-token", "("}}},
		{")", []tok{{")-token", ")"}}},
		{"{", []tok{{"{-token", "{"}}},
		{"}", []tok{{"}-token", "}"}}},
		{"/*c*/", []tok{{"comment-token", "/*c*/"}}},
		{"/*", []tok{{"comment-token", "/*"}}},
		{"/**/", []tok{{"comment-token", "/**/"}}},
		{"/", []tok{{"delim-token", "/"}}},
		{"\\", []tok{{"delim-token", "\\"}}},
		{"\\\n", []tok{{"delim-token", "\\"}, {"whitespace-token", "\n"}}},
		{"\u0000", []tok{{"delim-token", "\u0000"}}},
		{"!", []tok{{"delim-token", "!"}}},
		{"~=", []tok{{"delim-token", "~"}, {"delim-token", "="}}},
		{"\uFEFFa{b:c}", []tok{{"ident-token", "a"}, {"{-token", "{"}, {"ident-token", "b"}, {"colon-token", ":"}, {"ident-token", "c"}, {"}-token", "}"}}},
		{"a{b:c}", []tok{{"ident-token", "a"}, {"{-token", "{"}, {"ident-token", "b"}, {"colon-token", ":"}, {"ident-token", "c"}, {"}-token", "}"}}},
		{".test\n{\n  prop: url(foo/bar.jpg) url( a\\(\\33 \\).\\ \\\"\\'test ) calc(1 + 1) \\x \\aa ;\n}<!--<-->\\\n", []tok{{"delim-token", "."}, {"ident-token", "test"}, {"whitespace-token", "\n"}, {"{-token", "{"}, {"whitespace-token", "\n  "}, {"ident-token", "prop"}, {"colon-token", ":"}, {"whitespace-token", " "}, {"url-token", "url(foo/bar.jpg)"}, {"whitespace-token", " "}, {"url-token", "url( a\\(\\33 \\).\\ \\\"\\'test )"}, {"whitespace-token", " "}, {"function-token", "calc("}, {"number-token", "1"}, {"whitespace-token", " "}, {"delim-token", "+"}, {"whitespace-token", " "}, {"number-token", "1"}, {")-token", ")"}, {"whitespace-token", " "}, {"ident-token", "\\x"}, {"whitespace-token", " "}, {"ident-token", "\\aa "}, {"semicolon-token", ";"}, {"whitespace-token", "\n"}, {"}-token", "}"}, {"CDO-token", "<!--"}, {"delim-token", "<"}, {"CDC-token", "-->"}, {"delim-token", "\\"}, {"whitespace-token", "\n"}}},
	}

	for _, test := range tests {
		got := collect(t, test.css)

		if len(got) != len(test.expected) {
			t.Errorf("%q: got %d tokens %v, want %d %v", test.css, len(got), got, len(test.expected), test.expected)
			continue
		}

		for i, want := range test.expected {
			if got[i] != want {
				t.Errorf("%q: token %d is %v, want %v", test.css, i, got[i], want)
			}
		}
	}
}

// TestTokenizeEmpty checks that an empty source and a source that is a byte order mark alone
// report no token at all.
func TestTokenizeEmpty(t *testing.T) {
	for _, css := range []string{"", "\uFEFF"} {
		count := 0
		Tokenize(jsutil.FromString(css), func(tokenType, start, end int) {
			count++
		})
		if count != 0 {
			t.Errorf("%q: got %d tokens, want none", css, count)
		}
	}
}

// TestTokenizeBOMOffsets checks that the first token starts after a byte order mark.
func TestTokenizeBOMOffsets(t *testing.T) {
	var starts, ends []int
	Tokenize(jsutil.FromString("\uFEFFa b"), func(tokenType, start, end int) {
		starts = append(starts, start)
		ends = append(ends, end)
	})

	wantStarts := []int{1, 2, 3}
	wantEnds := []int{2, 3, 4}

	for i := range wantStarts {
		if i >= len(starts) || starts[i] != wantStarts[i] || ends[i] != wantEnds[i] {
			t.Fatalf("got starts %v ends %v, want %v and %v", starts, ends, wantStarts, wantEnds)
		}
	}
}

// TestTokenizeNilSource checks that a nil source is read as an empty one.
func TestTokenizeNilSource(t *testing.T) {
	Tokenize(nil, func(tokenType, start, end int) {
		t.Fatalf("got a token %d..%d of an empty source", start, end)
	})
}
