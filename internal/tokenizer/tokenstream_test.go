package tokenizer

import (
	"strings"
	"testing"

	"github.com/csstree/csstree/internal/jsutil"
)

const streamCSS = ".test\n{\n  prop: url(foo/bar.jpg) url( a\\(\\33 \\).\\ \\\"\\'test ) calc(1 + 1) \\x \\aa ;\n}<!--<-->\\\n"

var streamTokens = []struct {
	name    string
	chunk   string
	balance int
}{
	{"delim-token", ".", 31},
	{"ident-token", "test", 31},
	{"whitespace-token", "\n", 31},
	{"{-token", "{", 25},
	{"whitespace-token", "\n  ", 25},
	{"ident-token", "prop", 25},
	{"colon-token", ":", 25},
	{"whitespace-token", " ", 25},
	{"url-token", "url(foo/bar.jpg)", 25},
	{"whitespace-token", " ", 25},
	{"url-token", "url( a\\(\\33 \\).\\ \\\"\\'test )", 25},
	{"whitespace-token", " ", 25},
	{"function-token", "calc(", 18},
	{"number-token", "1", 18},
	{"whitespace-token", " ", 18},
	{"delim-token", "+", 18},
	{"whitespace-token", " ", 18},
	{"number-token", "1", 18},
	{")-token", ")", 12},
	{"whitespace-token", " ", 25},
	{"ident-token", "\\x", 25},
	{"whitespace-token", " ", 25},
	{"ident-token", "\\aa ", 25},
	{"semicolon-token", ";", 25},
	{"whitespace-token", "\n", 25},
	{"}-token", "}", 3},
	{"CDO-token", "<!--", 31},
	{"delim-token", "<", 31},
	{"CDC-token", "-->", 31},
	{"delim-token", "\\", 31},
	{"whitespace-token", "\n", 31},
}

func newStream(css string) *TokenStream {
	return NewTokenStream(jsutil.FromString(css), Tokenize)
}

// TestTokenStreamEmpty checks the state of a stream over a source with no token.
func TestTokenStreamEmpty(t *testing.T) {
	for _, css := range []string{"", "\ufeff"} {
		stream := newStream(css)

		if !stream.EOF {
			t.Errorf("%q: the stream is not at its end", css)
		}
		if stream.TokenType != EOF {
			t.Errorf("%q: the token type is %d, want %d", css, stream.TokenType, EOF)
		}
		if stream.TokenCount != 0 {
			t.Errorf("%q: the token count is %d, want 0", css, stream.TokenCount)
		}
	}
}

// TestTokenStreamNilTokenize checks that a stream built without a tokenizer holds no token.
func TestTokenStreamNilTokenize(t *testing.T) {
	stream := NewTokenStream(jsutil.FromString(streamCSS), nil)

	if !stream.EOF || stream.TokenCount != 0 {
		t.Fatalf("got eof %v and %d tokens, want true and 0", stream.EOF, stream.TokenCount)
	}
}

// TestTokenStreamForEachToken checks the type, text and balance of every token of a source that
// holds a block, a function, urls, escapes and the comment delimiters.
func TestTokenStreamForEachToken(t *testing.T) {
	stream := newStream(streamCSS)

	if stream.TokenCount != len(streamTokens) {
		t.Fatalf("got %d tokens, want %d", stream.TokenCount, len(streamTokens))
	}

	stream.ForEachToken(func(tokenType, start, end, index int) {
		want := streamTokens[index]

		if Names[tokenType] != want.name {
			t.Errorf("token %d is a %s, want a %s", index, Names[tokenType], want.name)
		}
		if chunk := stream.Source.Substring(start, end).String(); chunk != want.chunk {
			t.Errorf("token %d holds %q, want %q", index, chunk, want.chunk)
		}
		if balance := int(stream.Balance[index]); balance != want.balance {
			t.Errorf("token %d has balance %d, want %d", index, balance, want.balance)
		}
	})
}

// TestTokenStreamNext checks the positions a walk reports and the end of the walk.
func TestTokenStreamNext(t *testing.T) {
	stream := newStream(streamCSS)
	offset := 0
	index := 0

	for !stream.EOF {
		want := streamTokens[index]

		if Names[stream.TokenType] != want.name {
			t.Fatalf("token %d is a %s, want a %s", index, Names[stream.TokenType], want.name)
		}
		if stream.TokenStart != offset {
			t.Fatalf("token %d starts at %d, want %d", index, stream.TokenStart, offset)
		}
		offset += len(jsutil.FromString(want.chunk))
		if stream.TokenEnd != offset {
			t.Fatalf("token %d ends at %d, want %d", index, stream.TokenEnd, offset)
		}
		if stream.TokenIndex != index {
			t.Fatalf("the index of token %d is %d", index, stream.TokenIndex)
		}

		stream.Next()
		index++
	}

	if index != len(streamTokens) {
		t.Fatalf("the walk reported %d tokens, want %d", index, len(streamTokens))
	}
	if stream.TokenStart != len(stream.Source) || stream.TokenEnd != len(stream.Source) {
		t.Fatalf("the end of the walk is at %d..%d, want %d", stream.TokenStart, stream.TokenEnd, len(stream.Source))
	}
}

// TestTokenStreamGetToken checks the accessors by index, including the indexes outside the
// stream, which the original bounds into it.
func TestTokenStreamGetToken(t *testing.T) {
	stream := newStream("a{b}")
	// a { b } with the stream on its first token.
	tests := []struct {
		index int
		start int
		end   int
		name  string
	}{
		{-1, 0, 1, "ident-token"},
		{0, 0, 1, "ident-token"},
		{1, 1, 2, "{-token"},
		{2, 2, 3, "ident-token"},
		{3, 3, 4, "}-token"},
		{4, 4, 4, "EOF-token"},
		{5, 4, 4, "EOF-token"},
	}

	for _, test := range tests {
		if got := stream.GetTokenStart(test.index); got != test.start {
			t.Errorf("token %d starts at %d, want %d", test.index, got, test.start)
		}
		if got := stream.GetTokenEnd(test.index); got != test.end {
			t.Errorf("token %d ends at %d, want %d", test.index, got, test.end)
		}
		if got := Names[stream.GetTokenType(test.index)]; got != test.name {
			t.Errorf("token %d is a %s, want a %s", test.index, got, test.name)
		}
	}
}

// TestTokenStreamLookup checks the lookups ahead of the current token.
func TestTokenStreamLookup(t *testing.T) {
	stream := newStream("a /* c */ b(")

	if got := stream.LookupType(0); got != Ident {
		t.Errorf("lookupType(0) is %d, want %d", got, Ident)
	}
	if got := stream.LookupType(1); got != WhiteSpace {
		t.Errorf("lookupType(1) is %d, want %d", got, WhiteSpace)
	}
	if got := stream.LookupType(10); got != EOF {
		t.Errorf("lookupType(10) is %d, want %d", got, EOF)
	}
	if got := stream.LookupTypeNonSC(1); got != Function {
		t.Errorf("lookupTypeNonSC(1) is %d, want %d", got, Function)
	}
	if got := stream.LookupOffsetNonSC(1); got != 4 {
		t.Errorf("lookupOffsetNonSC(1) is %d, want 4", got)
	}
	if got := stream.LookupOffset(1); got != 1 {
		t.Errorf("lookupOffset(1) is %d, want 1", got)
	}
	// The comparison lower cases the source, never the reference, so a reference with an upper
	// case letter never matches, which is what the original does.
	if stream.LookupValue(0, "A") {
		t.Error("lookupValue(0, \"A\") is true, want false: only the source is lower cased")
	}
	if stream.LookupValue(1, "a") {
		t.Error("lookupValue(1, \"a\") is true, want false")
	}
	if !stream.IsDelim('a'-'a'+0x61, 0) == true {
		// a delim-token is not an ident-token, so this reads false
		t.Skip()
	}
	if stream.IsDelim(0x61, 0) {
		t.Error("the first token reads as a delim-token")
	}
}

// TestTokenStreamIsDelim checks the delim test on the current token and ahead of it.
func TestTokenStreamIsDelim(t *testing.T) {
	stream := newStream("& |")

	if !stream.IsDelim('&', 0) {
		t.Error("the current token is not the delim-token &")
	}
	if stream.IsDelim('|', 0) {
		t.Error("the current token reads as the delim-token |")
	}
	if !stream.IsDelim('|', 2) {
		t.Error("the token two ahead is not the delim-token |")
	}
}

// TestTokenStreamSubstrToCursor checks the text up to the current token.
func TestTokenStreamSubstrToCursor(t *testing.T) {
	stream := newStream("abc def")
	stream.Next()
	stream.Next()

	if got := stream.SubstrToCursor(0).String(); got != "abc " {
		t.Errorf("got %q, want %q", got, "abc ")
	}
}

// TestTokenStreamSkip checks a skip inside the stream and a skip past its end.
func TestTokenStreamSkip(t *testing.T) {
	stream := newStream(streamCSS)
	stream.Skip(3)

	if Names[stream.TokenType] != "{-token" {
		t.Errorf("after skipping 3 tokens the stream is on a %s, want a {-token", Names[stream.TokenType])
	}
	if stream.TokenStart != 6 || stream.TokenEnd != 7 {
		t.Errorf("after skipping 3 tokens the token is %d..%d, want 6..7", stream.TokenStart, stream.TokenEnd)
	}

	stream.Skip(len(streamTokens))

	if !stream.EOF {
		t.Error("a skip past the last token did not reach the end of the stream")
	}
}

// TestTokenStreamSkipSC checks that whitespace and comments are skipped.
func TestTokenStreamSkipSC(t *testing.T) {
	stream := newStream("  /* c */\t a")
	stream.SkipSC()

	if stream.TokenType != Ident || stream.TokenStart != 11 {
		t.Errorf("the stream stopped on token type %d at %d, want %d at 11", stream.TokenType, stream.TokenStart, Ident)
	}
}

// TestTokenStreamBalance checks the balance table, which pairs every block token with the token
// that closes its block and points every other token at the end of the block it is in. The values
// are the ones the original builds for the same source.
func TestTokenStreamBalance(t *testing.T) {
	stream := newStream("a([b])c")
	// a( [ b ] ) c <eof>
	want := []int{4, 3, 3, 1, 0, 6, 6}

	for i, w := range want {
		if got := int(stream.Balance[i]); got != w {
			t.Errorf("balance[%d] is %d, want %d", i, got, w)
		}
	}
}

// TestTokenStreamGetBlockTokenPairIndex checks the paired block tokens of a balanced source and
// of a source whose blocks do not close.
func TestTokenStreamGetBlockTokenPairIndex(t *testing.T) {
	tests := []struct {
		css      string
		expected []int
	}{
		{"start fn({[()]}) end", []int{-1, -1, 9, 8, 7, 6, 5, 4, 3, 2, -1, -1}},
		{"start fn(]}()[{)", []int{-1, -1, -1, -1, -1, 6, 5, -1, -1, -1}},
	}

	for _, test := range tests {
		stream := newStream(test.css)
		var got []int

		stream.ForEachToken(func(tokenType, start, end, index int) {
			got = append(got, stream.GetBlockTokenPairIndex(index))
		})

		if len(got) != len(test.expected) {
			t.Fatalf("%q: got %d tokens, want %d", test.css, len(got), len(test.expected))
		}
		for i := range got {
			if got[i] != test.expected[i] {
				t.Errorf("%q: the pair of token %d is %d, want %d", test.css, i, got[i], test.expected[i])
			}
		}
	}
}

// TestTokenStreamIsBlockTokenType checks which token types open and close a block.
func TestTokenStreamIsBlockTokenType(t *testing.T) {
	stream := newStream("")

	for _, tokenType := range []int{Function, LeftParenthesis, LeftSquareBracket, LeftCurlyBracket} {
		if !stream.IsBlockOpenerTokenType(tokenType) {
			t.Errorf("%s does not open a block", Names[tokenType])
		}
		if stream.IsBlockCloserTokenType(tokenType) {
			t.Errorf("%s closes a block", Names[tokenType])
		}
	}
	for _, tokenType := range []int{RightParenthesis, RightSquareBracket, RightCurlyBracket} {
		if !stream.IsBlockCloserTokenType(tokenType) {
			t.Errorf("%s does not close a block", Names[tokenType])
		}
	}
	for _, tokenType := range []int{EOF, Ident, Delim, Comment} {
		if stream.IsBlockOpenerTokenType(tokenType) || stream.IsBlockCloserTokenType(tokenType) {
			t.Errorf("%s reads as a block token", Names[tokenType])
		}
	}
}

// TestTokenStreamIsBalanceEdge checks the edge of the block the current token is in: a token
// index is past the edge when the balance entry of the token the stream is on is below it. The
// table is the one the original answers with for the same source.
func TestTokenStreamIsBalanceEdge(t *testing.T) {
	stream := newStream("a([b])c")
	want := []string{
		"0000011",
		"0000111",
		"0000111",
		"0011111",
		"0111111",
		"0000000",
	}

	for at, row := range want {
		stream.Reset()
		stream.Next()
		stream.Skip(at)

		for i := 0; i <= stream.TokenCount; i++ {
			got := byte('0')
			if stream.IsBalanceEdge(i) {
				got = '1'
			}
			if got != row[i] {
				t.Errorf("at token %d, isBalanceEdge(%d) is %c, want %c", at, i, got, row[i])
			}
		}
	}
}

// TestTokenStreamSkipUntilBalanced walks the cases of the original's own raw value tests: the
// scan stops on an unbalanced token, on the code point the caller refuses, or after it.
func TestTokenStreamSkipUntilBalanced(t *testing.T) {
	leftCurlyBracket := func(code uint16) int {
		if code == 0x007B {
			return 1
		}
		return 0
	}
	semicolonIncluded := func(code uint16) int {
		if code == 0x003B {
			return 2
		}
		return 0
	}
	never := func(uint16) int { return 0 }

	tests := []struct {
		source   string
		start    string
		skip     string
		mode     func(uint16) int
		expected string
	}{
		{"? { }", "^", "^", leftCurlyBracket, "? "},
		{"div { }", "^", "^", leftCurlyBracket, "div "},
		{"foo(bar(1)(2)(3[{}])(4{}){}(5))", "             ^", "             ^", leftCurlyBracket, "(3[{}])(4{})"},
		{"foo(bar(1) (2) (3[{}]) (4{}) {} (5))", "               ^", "                ^", leftCurlyBracket, "(3[{}]) (4{}) "},
		{"func(a func(;))", "     ^", "       ^", semicolonIncluded, "a func(;)"},
		{"func(a func(;))", "     ^", "            ^", semicolonIncluded, "a func(;)"},
		{"func(a func(;); b)", "     ^", "       ^", semicolonIncluded, "a func(;);"},
		{"func()", "     ^", "     ^", never, ""},
		{"func([{}])", "      ^", "       ^", never, "{}"},
		{"func([{})", "     ^", "      ^", never, "[{})"},
		{"func(1, 2, 3) {}", "^", "      ^", never, "func(1, 2, 3) {}"},
	}

	for i, test := range tests {
		stream := newStream(test.source)
		startOffset := strings.IndexByte(test.start, '^')
		skipToOffset := strings.IndexByte(test.skip, '^')
		startToken := stream.TokenIndex

		for stream.TokenStart < startOffset {
			stream.Next()
			startToken = stream.TokenIndex
		}
		for stream.TokenStart < skipToOffset {
			stream.Next()
		}

		stream.SkipUntilBalanced(startToken, test.mode)

		if got := stream.Source.Substring(startOffset, stream.TokenStart).String(); got != test.expected {
			t.Errorf("case %d: got %q, want %q", i, got, test.expected)
		}
	}
}

// TestTokenStreamReuse checks that a stream reads a second source correctly, with the buffers of
// the first one.
func TestTokenStreamReuse(t *testing.T) {
	stream := newStream(streamCSS)
	stream.SetSource(jsutil.FromString("a{}"), Tokenize)

	if stream.TokenCount != 3 {
		t.Fatalf("got %d tokens, want 3", stream.TokenCount)
	}
	if got := int(stream.Balance[0]); got != 3 {
		t.Errorf("balance[0] is %d, want 3", got)
	}
	if got := int(stream.Balance[1]); got != 2 {
		t.Errorf("balance[1] is %d, want 2", got)
	}
}

// TestTokenStreamDynamicBuffer checks a source with more tokens than the smallest buffer holds.
func TestTokenStreamDynamicBuffer(t *testing.T) {
	size := len(newStream(streamCSS).offsetAndType) + 10
	stream := newStream(strings.Repeat(".", size))
	count := 0

	for !stream.EOF {
		count++
		stream.Next()
	}

	if count != size {
		t.Fatalf("the walk reported %d tokens, want %d", count, size)
	}
	if len(stream.offsetAndType) < size {
		t.Fatalf("the buffer holds %d entries, want at least %d", len(stream.offsetAndType), size)
	}
}

// TestTokenStreamReset checks that a reset puts the stream back before its first token.
func TestTokenStreamReset(t *testing.T) {
	stream := newStream("a b")
	stream.Next()
	stream.Reset()

	if stream.TokenIndex != -1 || stream.EOF || stream.TokenType != 0 {
		t.Fatalf("after a reset the stream is at index %d, eof %v, type %d", stream.TokenIndex, stream.EOF, stream.TokenType)
	}

	stream.Next()

	if stream.TokenType != Ident || stream.TokenStart != 0 {
		t.Fatalf("after a reset the first token is type %d at %d", stream.TokenType, stream.TokenStart)
	}
}
