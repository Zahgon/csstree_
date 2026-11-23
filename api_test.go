package csstree_test

import (
	"strings"
	"testing"

	csstree "github.com/csstree/csstree"
)

// TestLibraryRoundTrip reads CSS into a tree and writes it back out, through the public API only.
func TestLibraryRoundTrip(t *testing.T) {
	tree, err := csstree.Parse(csstree.Text("a { color: red }"), csstree.NewParseOptions())
	if err != nil {
		t.Fatalf("parsing failed: %s", err.Message.String())
	}

	css, err := csstree.Generate(tree, "")
	if err != nil {
		t.Fatalf("generating failed: %s", err.Message.String())
	}

	if got, want := css.String(), "a{color:red}"; got != want {
		t.Errorf("the CSS is %q, want %q", got, want)
	}
}

// TestLibraryWalk counts the declarations of a style sheet through the public API.
func TestLibraryWalk(t *testing.T) {
	tree, err := csstree.Parse(csstree.Text("a{b:c;d:e}f{g:h}"), csstree.NewParseOptions())
	if err != nil {
		t.Fatalf("parsing failed: %s", err.Message.String())
	}

	declarations := csstree.FindAll(tree, func(node *csstree.Node, _ *csstree.Item, _ *csstree.List, _ *csstree.WalkContext) bool {
		return node.Type == "Declaration"
	})

	if len(declarations) != 3 {
		t.Errorf("found %d declarations, want 3", len(declarations))
	}
}

// TestLibraryTokenize reads the token stream through the public API.
func TestLibraryTokenize(t *testing.T) {
	var names []string
	csstree.Tokenize(csstree.Text("a{}"), func(tokenType, start, end int) {
		names = append(names, csstree.TokenName(tokenType))
	})

	if got, want := strings.Join(names, " "), "ident-token {-token }-token"; got != want {
		t.Errorf("the tokens are %q, want %q", got, want)
	}
}

// TestLibraryLexer matches a declaration value against the CSS grammar through the public API.
func TestLibraryLexer(t *testing.T) {
	lexer, err := csstree.DefaultLexer()
	if err != nil {
		t.Fatalf("building the lexer failed: %v", err)
	}

	if result := lexer.MatchProperty(csstree.Text("color"), csstree.TextValue(csstree.Text("red"))); result.Matched == nil {
		t.Error("color: red did not match")
	}

	result := lexer.MatchProperty(csstree.Text("color"), csstree.TextValue(csstree.Text("1px")))
	if result.Matched != nil {
		t.Error("color: 1px matched and should not have")
	}
	if result.Error == nil || result.Error.Name != "SyntaxMatchError" {
		t.Error("color: 1px did not report a match error")
	}
}

// TestLibraryJSON writes a tree as JSON through the public API.
func TestLibraryJSON(t *testing.T) {
	tree, err := csstree.Parse(csstree.Text("a"), csstree.ParseOptions{Context: "selector"})
	if err != nil {
		t.Fatalf("parsing failed: %s", err.Message.String())
	}

	text := csstree.JSON(csstree.ToPlainObject(tree), 0).String()
	want := `{"type":"Selector","loc":null,"children":[{"type":"TypeSelector","loc":null,"name":"a"}]}`

	if text != want {
		t.Errorf("the JSON is %s, want %s", text, want)
	}
}

// TestLibraryDefinitionSyntax reads and writes a value definition syntax through the public API.
func TestLibraryDefinitionSyntax(t *testing.T) {
	definition, err := csstree.ParseDefinition(csstree.Text("<length> | auto"))
	if err != nil {
		t.Fatalf("reading the definition failed: %s", err.Message.String())
	}

	compact, genErr := csstree.GenerateDefinition(definition, false, true)
	if genErr != nil {
		t.Fatalf("writing the definition failed: %s", genErr.Message.String())
	}
	if got, want := compact.String(), "<length>|auto"; got != want {
		t.Errorf("the compact form is %q, want %q", got, want)
	}

	// A tree the writer does not know is reported rather than left to unwind.
	if _, err := csstree.GenerateDefinition(&csstree.Definition{Type: "Nope"}, false, false); err == nil {
		t.Error("an unknown definition node was written without an error")
	}
	if err := csstree.WalkDefinition(&csstree.Definition{Type: "Nope"}, func(*csstree.Definition) {}, nil); err == nil {
		t.Error("an unknown definition node was walked without an error")
	}
}

// TestLibraryUtilities exercises the encoding helpers through the public API.
func TestLibraryUtilities(t *testing.T) {
	if got := csstree.DecodeString(csstree.Text(`"a\62 c"`)).String(); got != "abc" {
		t.Errorf("DecodeString is %q", got)
	}
	if got := csstree.EncodeIdent(csstree.Text("1a")).String(); got != `\31 a` {
		t.Errorf("EncodeIdent is %q", got)
	}
	if !csstree.IsCustomProperty(csstree.Text("--x"), 0) {
		t.Error("--x is not read as a custom property")
	}
}
