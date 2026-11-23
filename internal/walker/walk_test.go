package walker

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
)

// corpusEntry is one tree of the recorded corpus, with the walk the original performs over it in
// each of the forms the corpus covers.
type corpusEntry struct {
	Name string            `json:"name"`
	AST  json.RawMessage   `json:"ast"`
	Runs map[string]string `json:"runs"`
}

func loadCorpus(t *testing.T) []corpusEntry {
	t.Helper()

	raw, err := os.ReadFile("testdata/corpus.json")
	if err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}

	var entries []corpusEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("the corpus is empty")
	}

	return entries
}

func treeOf(t *testing.T, entry corpusEntry) *ast.Node {
	t.Helper()

	value, err := jsutil.ParseJSONBytes(entry.AST)
	if err != nil {
		t.Fatalf("%s: reading the tree: %v", entry.Name, err)
	}

	return ast.FromPlain(value)
}

// record writes the walk the way the corpus records it: one line per callback with the depth, the
// node type, whether the node is a list item, and the nine walk context fields.
func record(tree *ast.Node, options Options, enteredAt map[*ast.Node]int) string {
	var lines []string

	contextOf := func(ctx *Context) string {
		fields := []*ast.Node{ctx.Root, ctx.StyleSheet, ctx.Atrule, ctx.AtrulePrelude, ctx.Rule,
			ctx.Selector, ctx.Block, ctx.Declaration, ctx.Function}
		parts := make([]string, len(fields))
		for i, field := range fields {
			if field == nil {
				parts[i] = "-"
				continue
			}
			parts[i] = fmt.Sprint(enteredAt[field])
		}
		return strings.Join(parts, ",")
	}

	options.Enter = func(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context) {
		visit := enteredAt[node]
		inList := "F"
		if item != nil {
			inList = "L"
		}
		lines = append(lines, fmt.Sprintf("enter %d %s %s %s", visit, node.Type, inList, contextOf(ctx)))
	}
	options.Leave = func(node *ast.Node, item *ast.Item, list *ast.List, ctx *Context) {
		inList := "F"
		if item != nil {
			inList = "L"
		}
		lines = append(lines, fmt.Sprintf("leave %d %s %s %s", enteredAt[node], node.Type, inList, contextOf(ctx)))
	}

	Walk(tree, options)

	return strings.Join(lines, "\n")
}

// TestWalkRuns walks every tree of the corpus in each recorded form and compares the order and
// the walk context with what the original produced.
func TestWalkRuns(t *testing.T) {
	forms := map[string]Options{
		"natural":                   {},
		"reverse":                   {Reverse: true},
		"visit-Rule":                {Visit: "Rule"},
		"visit-Declaration":         {Visit: "Declaration"},
		"visit-Atrule":              {Visit: "Atrule"},
		"visit-Identifier":          {Visit: "Identifier"},
		"visit-Rule-reverse":        {Visit: "Rule", Reverse: true},
		"visit-Declaration-reverse": {Visit: "Declaration", Reverse: true},
	}

	checked := 0

	for _, entry := range loadCorpus(t) {
		tree := treeOf(t, entry)
		// Every node is numbered once by a natural walk; the recorded runs name a node by that
		// number whichever order they walk in.
		enteredAt := map[*ast.Node]int{}
		next := 0
		Walk(tree, Options{Enter: func(node *ast.Node, _ *ast.Item, _ *ast.List, _ *Context) {
			enteredAt[node] = next
			next++
		}})

		for form, options := range forms {
			want, ok := entry.Runs[form]
			if !ok {
				continue
			}

			if got := record(tree, options, enteredAt); got != want {
				t.Errorf("%s (%s):\n got\n%s\n want\n%s", entry.Name, form, got, want)
			}

			checked++
		}
	}

	t.Logf("%d walks matched", checked)
}

// TestBadVisit checks the error an unknown node type raises as the visit option.
func TestBadVisit(t *testing.T) {
	tree := ast.New("StyleSheet")
	tree.Set("children", ast.NewList())

	err := Walk(tree, Options{Visit: "Nope", Enter: func(*ast.Node, *ast.Item, *ast.List, *Context) {}})
	if err == nil {
		t.Fatal("an unknown visit type was accepted")
	}
	if !strings.HasPrefix(err.Message.String(), "Bad value `Nope` for `visit` option") {
		t.Errorf("the message is %q", err.Message.String())
	}
	if !strings.Contains(err.Message.String(), "AnPlusB, Atrule, AtrulePrelude") {
		t.Error("the message does not list the node types")
	}
}

// TestFind checks the three searches over a small tree.
func TestFind(t *testing.T) {
	value := ast.New("Value")
	children := ast.NewList()
	for _, name := range []string{"a", "b", "c"} {
		node := ast.New("Identifier")
		node.SetString("name", name)
		children.AppendData(node)
	}
	value.Set("children", children)

	isIdentifier := func(node *ast.Node, _ *ast.Item, _ *ast.List, _ *Context) bool {
		return node.Type == "Identifier"
	}

	if node := Find(value, isIdentifier); node == nil || node.StrOrEmpty("name") != "a" {
		t.Error("find did not return the first identifier")
	}
	if node := FindLast(value, isIdentifier); node == nil || node.StrOrEmpty("name") != "c" {
		t.Error("findLast did not return the last identifier")
	}
	if nodes := FindAll(value, isIdentifier); len(nodes) != 3 {
		t.Errorf("findAll returned %d identifiers, want 3", len(nodes))
	}
}
