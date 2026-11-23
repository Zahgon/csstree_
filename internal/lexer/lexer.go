package lexer

import (
	"sort"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/cssutil"
	"github.com/csstree/csstree/internal/dict"
	"github.com/csstree/csstree/internal/generator"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/syntaxdef"
	"github.com/csstree/csstree/internal/tokenizer"
)

// Atrule is one at-rule of the lexer's dictionary.
type Atrule struct {
	Name        string
	Prelude     *Descriptor
	Descriptors map[string]*Descriptor
	// DescriptorNames keeps the order the descriptors were added in, which is the order they are
	// written out in.
	DescriptorNames []string
}

// Lexer matches CSS values against the CSS grammar.
type Lexer struct {
	Generic         bool
	CSSWideKeywords []string
	Units           map[string][]string

	types      map[string]*Descriptor
	typeNames  []string
	properties map[string]*Descriptor
	propNames  []string
	atrules    map[string]*Atrule
	atruleName []string

	cssWideKeywordsSyntax *matchNode
}

// New builds a lexer over a resolved dictionary, with the generic types the units allow.
func New(d *dict.Dict) *Lexer {
	l := &Lexer{
		Generic:         true,
		CSSWideKeywords: cssWideKeywords,
		Units:           defaultUnits,
		types:           map[string]*Descriptor{},
		properties:      map[string]*Descriptor{},
		atrules:         map[string]*Atrule{},
	}

	// The dictionary's own types are added first and the generic types after them, so that a type
	// the lexer matches with code of its own wins over a definition of the same name, which is
	// the order the original adds them in.
	for _, key := range d.Types.Keys() {
		value, _ := d.Types.Get(key)
		if text, ok := value.(jsutil.JSONString); ok {
			name := key.String()
			l.addType(name, newDescriptor(jsutil.Str(text), "Type", name, ""))
		}
	}

	genericNames := make([]string, 0, 64)
	generics := createGenericTypes(l.Units)
	for name := range generics {
		genericNames = append(genericNames, name)
	}
	sort.Strings(genericNames)
	for _, name := range genericNames {
		l.addType(name, newGenericDescriptor(generics[name], "Type", name))
	}

	for _, key := range d.Atrules.Keys() {
		entry, _ := d.Atrules.Get(key)
		name := key.String()
		atrule := &Atrule{Name: name, Descriptors: map[string]*Descriptor{}}

		if entry.HasPrelude {
			atrule.Prelude = newDescriptor(entry.Prelude, "AtrulePrelude", name, "")
		}

		if entry.Descriptors != nil {
			for _, descKey := range entry.Descriptors.Keys() {
				value, _ := entry.Descriptors.Get(descKey)
				text, ok := value.(jsutil.JSONString)
				if !ok {
					continue
				}
				descName := descKey.String()
				atrule.Descriptors[descName] = newDescriptor(jsutil.Str(text), "AtruleDescriptor", descName, name)
				atrule.DescriptorNames = append(atrule.DescriptorNames, descName)
			}
		}

		// An at-rule whose descriptor map is empty has no descriptors at all, which is what the
		// configuration assembly of the original leaves behind.
		if len(atrule.DescriptorNames) == 0 {
			atrule.Descriptors = nil
		}

		if _, taken := l.atrules[name]; !taken {
			l.atruleName = append(l.atruleName, name)
		}
		l.atrules[name] = atrule
	}

	for _, key := range d.Properties.Keys() {
		value, _ := d.Properties.Get(key)
		if text, ok := value.(jsutil.JSONString); ok {
			name := key.String()
			if _, taken := l.properties[name]; !taken {
				l.propNames = append(l.propNames, name)
			}
			l.properties[name] = newDescriptor(jsutil.Str(text), "Property", name, "")
		}
	}

	keywords := jsutil.Str{}
	for i, keyword := range l.CSSWideKeywords {
		if i > 0 {
			keywords = append(keywords, jsutil.FromString(" |  ")...)
		}
		keywords = append(keywords, jsutil.FromString(keyword)...)
	}
	graph, err := buildMatchGraphFromText(keywords, nil)
	if err == nil {
		l.cssWideKeywordsSyntax = graph
	}

	return l
}

func (l *Lexer) addType(name string, d *Descriptor) {
	if _, taken := l.types[name]; !taken {
		l.typeNames = append(l.typeNames, name)
	}
	l.types[name] = d
}

// dictionary gives the matcher the two maps it looks a named syntax up in.
func (l *Lexer) dictionary() *dictionary {
	types := map[string]*dictEntry{}
	for name, descriptor := range l.types {
		types[name] = l.entryOf(descriptor)
	}
	properties := map[string]*dictEntry{}
	for name, descriptor := range l.properties {
		properties[name] = l.entryOf(descriptor)
	}
	return &dictionary{types: types, properties: properties, hasTypes: true, hasProps: true}
}

// entryOf builds a descriptor's graph when the matcher first needs it.
func (l *Lexer) entryOf(d *Descriptor) *dictEntry {
	match, err := d.Match()
	if err != nil {
		return &dictEntry{}
	}
	return &dictEntry{match: match, matchRef: d.matchRef}
}

// MatchResult is what a match left behind: the tree of syntaxes that matched, or the error that
// says why nothing did.
type MatchResult struct {
	Matched    *MatchTree
	Error      *jsutil.Error
	Iterations int
}

func buildMatchResult(matched *MatchTree, err *jsutil.Error, iterations int) *MatchResult {
	return &MatchResult{Matched: matched, Error: err, Iterations: iterations}
}

// valueHasVar reports whether a value calls var(), which the matcher cannot see through.
func valueHasVar(tokens []generator.Token) bool {
	for i := range tokens {
		if tokens[i].Value.ToLower().EqualString("var(") {
			return true
		}
	}
	return false
}

// Value is a value to match: either CSS text or a tree.
type Value struct {
	Text   jsutil.Str
	Node   *ast.Node
	IsText bool
}

// TextValue wraps CSS text.
func TextValue(text jsutil.Str) Value { return Value{Text: text, IsText: true} }

// NodeValue wraps a tree.
func NodeValue(node *ast.Node) Value { return Value{Node: node} }

// prepareTokens turns a value into the tokens the matcher walks, tagging each with the node it
// came from when the value is a tree.
func prepareTokens(value Value) ([]generator.Token, *jsutil.Error) {
	if value.IsText {
		tokens := []generator.Token{}
		tokenizer.Tokenize(value.Text, func(tokenType, start, end int) {
			tokens = append(tokens, generator.Token{Type: tokenType, Value: value.Text.Substring(start, end)})
		})
		return tokens, nil
	}

	return generator.Tokens(value.Node)
}

// matchSyntax walks a value through one descriptor's graph.
func (l *Lexer) matchSyntax(syntax *Descriptor, value Value, useCSSWideKeywords bool) *MatchResult {
	tokens, err := prepareTokens(value)
	if err != nil {
		return buildMatchResult(nil, err, 0)
	}

	if valueHasVar(tokens) {
		return buildMatchResult(nil, jsutil.NewError("Error",
			jsutil.FromString("Matching for a tree with var() is not supported")), 0)
	}

	dictionary := l.dictionary()

	var result *internalResult
	var tree *MatchTree

	if useCSSWideKeywords && l.cssWideKeywordsSyntax != nil {
		result, tree, err = matchAsTree(tokens, l.cssWideKeywordsSyntax, dictionary)
		if err != nil {
			return buildMatchResult(nil, err, 0)
		}
	}

	if !useCSSWideKeywords || tree == nil {
		graph, graphErr := syntax.Match()
		if graphErr != nil {
			return buildMatchResult(nil, graphErr, 0)
		}

		result, tree, err = matchAsTree(tokens, graph, dictionary)
		if err != nil {
			return buildMatchResult(nil, err, 0)
		}

		if tree == nil {
			definition, _ := syntax.Syntax()
			return buildMatchResult(nil, newSyntaxMatchError(result.reason, definition, value, result), result.iterations)
		}
	}

	return buildMatchResult(tree, nil, result.iterations)
}

// GetAtrule returns the at-rule of a name, falling back to the name without its vendor prefix.
func (l *Lexer) GetAtrule(atruleName jsutil.Str, fallbackBasename bool) *Atrule {
	descriptor := cssutil.Keyword(atruleName)

	if len(descriptor.Vendor) > 0 && fallbackBasename {
		if entry, ok := l.atrules[descriptor.Name.String()]; ok {
			return entry
		}
		if entry, ok := l.atrules[descriptor.Basename.String()]; ok {
			return entry
		}
		return nil
	}

	if entry, ok := l.atrules[descriptor.Name.String()]; ok {
		return entry
	}
	return nil
}

// GetAtrulePrelude returns the prelude definition of an at-rule.
func (l *Lexer) GetAtrulePrelude(atruleName jsutil.Str, fallbackBasename bool) *Descriptor {
	atrule := l.GetAtrule(atruleName, fallbackBasename)
	if atrule == nil {
		return nil
	}
	return atrule.Prelude
}

// GetProperty returns the definition of a property, falling back to the name without its vendor
// prefix.
func (l *Lexer) GetProperty(propertyName jsutil.Str, fallbackBasename bool) *Descriptor {
	descriptor := cssutil.Property(propertyName)

	if len(descriptor.Vendor) > 0 && fallbackBasename {
		if entry, ok := l.properties[descriptor.Name.String()]; ok {
			return entry
		}
		if entry, ok := l.properties[descriptor.Basename.String()]; ok {
			return entry
		}
		return nil
	}

	if entry, ok := l.properties[descriptor.Name.String()]; ok {
		return entry
	}
	return nil
}

// GetType returns the definition of a type.
func (l *Lexer) GetType(name jsutil.Str) *Descriptor {
	if entry, ok := l.types[name.String()]; ok {
		return entry
	}
	return nil
}

// CheckAtruleName reports an at-rule the dictionary does not know.
func (l *Lexer) CheckAtruleName(atruleName jsutil.Str) *jsutil.Error {
	if l.GetAtrule(atruleName, true) == nil {
		return newSyntaxReferenceError("Unknown at-rule", jsutil.Concat(jsutil.FromString("@"), atruleName))
	}
	return nil
}

// CheckAtrulePrelude reports a prelude an at-rule should or should not have.
func (l *Lexer) CheckAtrulePrelude(atruleName, prelude jsutil.Str, hasPrelude bool) *jsutil.Error {
	if err := l.CheckAtruleName(atruleName); err != nil {
		return err
	}

	atrule := l.GetAtrule(atruleName, true)

	if atrule.Prelude == nil && hasPrelude && len(prelude) > 0 {
		return jsutil.NewError("SyntaxError", jsutil.Concat(
			jsutil.FromString("At-rule `@"), atruleName, jsutil.FromString("` should not contain a prelude")))
	}

	if atrule.Prelude != nil && (!hasPrelude || len(prelude) == 0) {
		if l.matchSyntax(atrule.Prelude, TextValue(jsutil.Str{}), false).Matched == nil {
			return jsutil.NewError("SyntaxError", jsutil.Concat(
				jsutil.FromString("At-rule `@"), atruleName, jsutil.FromString("` should contain a prelude")))
		}
	}

	return nil
}

// CheckAtruleDescriptorName reports a descriptor an at-rule does not have.
func (l *Lexer) CheckAtruleDescriptorName(atruleName, descriptorName jsutil.Str) *jsutil.Error {
	if err := l.CheckAtruleName(atruleName); err != nil {
		return err
	}

	atrule := l.GetAtrule(atruleName, true)
	descriptor := cssutil.Keyword(descriptorName)

	if atrule.Descriptors == nil {
		return jsutil.NewError("SyntaxError", jsutil.Concat(
			jsutil.FromString("At-rule `@"), atruleName, jsutil.FromString("` has no known descriptors")))
	}

	if atrule.Descriptors[descriptor.Name.String()] == nil && atrule.Descriptors[descriptor.Basename.String()] == nil {
		return newSyntaxReferenceError("Unknown at-rule descriptor", descriptorName)
	}

	return nil
}

// CheckPropertyName reports a property the dictionary does not know.
func (l *Lexer) CheckPropertyName(propertyName jsutil.Str) *jsutil.Error {
	if l.GetProperty(propertyName, true) == nil {
		return newSyntaxReferenceError("Unknown property", propertyName)
	}
	return nil
}

// MatchAtrulePrelude matches an at-rule's prelude against its definition.
func (l *Lexer) MatchAtrulePrelude(atruleName jsutil.Str, prelude Value) *MatchResult {
	preludeText := prelude.Text
	if !prelude.IsText {
		preludeText = jsutil.FromString("tree")
	}

	if err := l.CheckAtrulePrelude(atruleName, preludeText, true); err != nil {
		return buildMatchResult(nil, err, 0)
	}

	atrule := l.GetAtrule(atruleName, true)

	if atrule.Prelude == nil {
		return buildMatchResult(nil, nil, 0)
	}

	if prelude.IsText && len(prelude.Text) == 0 {
		prelude = TextValue(jsutil.Str{})
	}

	return l.matchSyntax(atrule.Prelude, prelude, false)
}

// MatchAtruleDescriptor matches a value against an at-rule descriptor's definition.
func (l *Lexer) MatchAtruleDescriptor(atruleName, descriptorName jsutil.Str, value Value) *MatchResult {
	if err := l.CheckAtruleDescriptorName(atruleName, descriptorName); err != nil {
		return buildMatchResult(nil, err, 0)
	}

	atrule := l.GetAtrule(atruleName, true)
	descriptor := cssutil.Keyword(descriptorName)

	entry := atrule.Descriptors[descriptor.Name.String()]
	if entry == nil {
		entry = atrule.Descriptors[descriptor.Basename.String()]
	}

	return l.matchSyntax(entry, value, false)
}

// MatchDeclaration matches a declaration's value against its property's definition.
func (l *Lexer) MatchDeclaration(node *ast.Node) *MatchResult {
	if node == nil || node.Type != "Declaration" {
		return buildMatchResult(nil, jsutil.NewError("Error", jsutil.FromString("Not a Declaration node")), 0)
	}

	return l.MatchProperty(node.Str("property"), NodeValue(node.Child("value")))
}

// MatchProperty matches a value against a property's definition.
func (l *Lexer) MatchProperty(propertyName jsutil.Str, value Value) *MatchResult {
	// a custom property has no definition to match against
	if cssutil.Property(propertyName).Custom {
		return buildMatchResult(nil, jsutil.NewError("Error",
			jsutil.FromString("Lexer matching doesn't applicable for custom properties")), 0)
	}

	if err := l.CheckPropertyName(propertyName); err != nil {
		return buildMatchResult(nil, err, 0)
	}

	return l.matchSyntax(l.GetProperty(propertyName, true), value, true)
}

// MatchType matches a value against a type's definition.
func (l *Lexer) MatchType(typeName jsutil.Str, value Value) *MatchResult {
	typeSyntax := l.GetType(typeName)

	if typeSyntax == nil {
		return buildMatchResult(nil, newSyntaxReferenceError("Unknown type", typeName), 0)
	}

	return l.matchSyntax(typeSyntax, value, false)
}

// Match matches a value against a definition given as text.
func (l *Lexer) Match(definition jsutil.Str, value Value) *MatchResult {
	return l.matchSyntax(newDescriptor(definition, "Type", "anonymous", ""), value, false)
}

// Dump writes the dictionary out: the generic flag, the CSS-wide keywords, the units, and the
// definition of every type, property and at-rule. pretty asks for the roomier form, which is the
// opposite of the compact one the generator takes.
func (l *Lexer) Dump(syntaxAsAst, pretty bool) jsutil.Value {
	compact := !pretty

	out := jsutil.NewObject()
	out.SetString("generic", jsutil.Bool(l.Generic))

	keywords := jsutil.Array{}
	for _, keyword := range l.CSSWideKeywords {
		keywords = append(keywords, jsutil.JSONString(jsutil.FromString(keyword)))
	}
	out.SetString("cssWideKeywords", keywords)

	// The units are written in the order a module namespace object gives its keys, which is
	// sorted, rather than in the order the groups are declared.
	sortedGroups := append([]string(nil), unitGroups...)
	sort.Strings(sortedGroups)

	units := jsutil.NewObject()
	for _, group := range sortedGroups {
		list := jsutil.Array{}
		for _, unit := range l.Units[group] {
			list = append(list, jsutil.JSONString(jsutil.FromString(unit)))
		}
		units.SetString(group, list)
	}
	out.SetString("units", units)

	out.SetString("types", l.dumpMap(l.typeNames, l.types, compact, syntaxAsAst))
	out.SetString("properties", l.dumpMap(l.propNames, l.properties, compact, syntaxAsAst))

	atrules := jsutil.NewObject()
	for _, name := range l.atruleName {
		atrule := l.atrules[name]
		entry := jsutil.NewObject()

		if atrule.Prelude == nil {
			entry.SetString("prelude", jsutil.Null{})
		} else {
			entry.SetString("prelude", l.dumpSyntax(atrule.Prelude, compact, syntaxAsAst))
		}

		if atrule.Descriptors == nil {
			entry.SetString("descriptors", jsutil.Null{})
		} else {
			entry.SetString("descriptors", l.dumpMap(atrule.DescriptorNames, atrule.Descriptors, compact, syntaxAsAst))
		}

		atrules.SetString(name, entry)
	}
	out.SetString("atrules", atrules)

	return out
}

func (l *Lexer) dumpMap(names []string, entries map[string]*Descriptor, compact, syntaxAsAst bool) jsutil.Value {
	out := jsutil.NewObject()
	for _, name := range names {
		descriptor := entries[name]
		if descriptor == nil {
			continue
		}
		syntax, err := descriptor.Syntax()
		if err != nil || syntax == nil {
			continue
		}
		out.SetString(name, l.dumpSyntax(descriptor, compact, syntaxAsAst))
	}
	return out
}

func (l *Lexer) dumpSyntax(descriptor *Descriptor, compact, syntaxAsAst bool) jsutil.Value {
	syntax, err := descriptor.Syntax()
	if err != nil || syntax == nil {
		return jsutil.Null{}
	}
	if syntaxAsAst {
		return syntaxdef.ToPlain(syntax)
	}
	return jsutil.JSONString(syntaxdef.Generate(syntax, false, compact))
}

// Validate walks every definition and reports the ones that name a definition that is missing or
// itself broken. It answers nothing when every definition resolves.
func (l *Lexer) Validate() jsutil.Value {
	errors := jsutil.Array{}
	brokenTypes := map[string]bool{}
	brokenProperties := map[string]bool{}
	var typeOrder, propertyOrder []string

	syntaxRef := func(name string, isType bool) string {
		if isType {
			return "<" + name + ">"
		}
		return "<'" + name + "'>"
	}

	var validate func(name string, broken map[string]bool, order *[]string, descriptor *Descriptor, isType bool) bool
	validate = func(name string, broken map[string]bool, order *[]string, descriptor *Descriptor, isType bool) bool {
		if value, seen := broken[name]; seen {
			return value
		}

		broken[name] = false
		*order = append(*order, name)

		syntax, err := descriptor.Syntax()
		if err != nil || syntax == nil {
			return broken[name]
		}

		syntaxdef.Walk(syntax, func(node *syntaxdef.Node) {
			if node.Type != "Type" && node.Type != "Property" {
				return
			}

			nodeName := node.Str("name").String()
			nodeIsType := node.Type == "Type"

			var entries map[string]*Descriptor
			var nestedBroken map[string]bool
			var nestedOrder *[]string
			if nodeIsType {
				entries, nestedBroken, nestedOrder = l.types, brokenTypes, &typeOrder
			} else {
				entries, nestedBroken, nestedOrder = l.properties, brokenProperties, &propertyOrder
			}

			entry, known := entries[nodeName]
			if !known {
				errors = append(errors, jsutil.JSONString(jsutil.FromString(
					syntaxRef(name, isType)+" used missed syntax definition "+syntaxRef(nodeName, nodeIsType))))
				broken[name] = true
			} else if validate(nodeName, nestedBroken, nestedOrder, entry, nodeIsType) {
				errors = append(errors, jsutil.JSONString(jsutil.FromString(
					syntaxRef(name, isType)+" used broken syntax definition "+syntaxRef(nodeName, nodeIsType))))
				broken[name] = true
			}
		}, nil)

		return broken[name]
	}

	for _, name := range l.typeNames {
		validate(name, brokenTypes, &typeOrder, l.types[name], true)
	}

	for _, name := range l.propNames {
		validate(name, brokenProperties, &propertyOrder, l.properties[name], false)
	}

	brokenTypeList := jsutil.Array{}
	for _, name := range typeOrder {
		if brokenTypes[name] {
			brokenTypeList = append(brokenTypeList, jsutil.JSONString(jsutil.FromString(name)))
		}
	}

	brokenPropertyList := jsutil.Array{}
	for _, name := range propertyOrder {
		if brokenProperties[name] {
			brokenPropertyList = append(brokenPropertyList, jsutil.JSONString(jsutil.FromString(name)))
		}
	}

	if len(brokenTypeList) > 0 || len(brokenPropertyList) > 0 {
		out := jsutil.NewObject()
		out.SetString("errors", errors)
		out.SetString("types", brokenTypeList)
		out.SetString("properties", brokenPropertyList)
		return out
	}

	return jsutil.Null{}
}
