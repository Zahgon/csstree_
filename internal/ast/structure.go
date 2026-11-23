package ast

// The declared structure of every node type, mirroring the node definitions of the original. The
// walker reads it to know which fields hold children, the structure check reads it to say what a
// field may hold, and the lexer reads it to describe a node type.

// FieldDef says what one field of a node type may hold.
type FieldDef struct {
	Name string
	// Declared, in the order they were declared: "string", "boolean", "null", a node type name, or
	// "List<...>" for a list of children.
	Declared []string
	// String, Boolean and Nullable say whether a plain string, a boolean or nothing is allowed.
	String   bool
	Boolean  bool
	Nullable bool
	// NodeTypes are the node types the field may hold directly; empty when it holds no single node.
	NodeTypes []string
	// IsList says whether the field may hold a list of children, and ListTypes the types those
	// children may have, empty meaning any.
	IsList    bool
	ListTypes []string
}

// TypeDef is the declared structure of one node type.
type TypeDef struct {
	Name string
	// WalkContext is the name of the walk context field this node type sets while it is entered.
	WalkContext string
	Fields      []FieldDef
}

// Types is every node type of the default syntax, keyed by name.
var Types = map[string]*TypeDef{}

// TypeNames are the node type names in the order the structure check and the walker report them.
var TypeNames = []string{
	"AnPlusB",
	"Atrule",
	"AtrulePrelude",
	"AttributeSelector",
	"Block",
	"Brackets",
	"CDC",
	"CDO",
	"ClassSelector",
	"Combinator",
	"Comment",
	"Condition",
	"Declaration",
	"DeclarationList",
	"Dimension",
	"Feature",
	"FeatureFunction",
	"FeatureRange",
	"Function",
	"GeneralEnclosed",
	"Hash",
	"IdSelector",
	"Identifier",
	"Layer",
	"LayerList",
	"MediaQuery",
	"MediaQueryList",
	"NestingSelector",
	"Nth",
	"Number",
	"Operator",
	"Parentheses",
	"Percentage",
	"PseudoClassSelector",
	"PseudoElementSelector",
	"Ratio",
	"Raw",
	"Rule",
	"Scope",
	"Selector",
	"SelectorList",
	"String",
	"StyleSheet",
	"SupportsDeclaration",
	"TypeSelector",
	"UnicodeRange",
	"Url",
	"Value",
	"WhiteSpace",
}

func init() {
	Types["AnPlusB"] = &TypeDef{Name: "AnPlusB", WalkContext: "", Fields: []FieldDef{
		{Name: "a", Declared: []string{"string", "null"}, String: true, Boolean: false, Nullable: true, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "b", Declared: []string{"string", "null"}, String: true, Boolean: false, Nullable: true, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Atrule"] = &TypeDef{Name: "Atrule", WalkContext: "atrule", Fields: []FieldDef{
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "prelude", Declared: []string{"AtrulePrelude", "Raw", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: []string{"AtrulePrelude", "Raw"}, IsList: false, ListTypes: nil},
		{Name: "block", Declared: []string{"Block", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: []string{"Block"}, IsList: false, ListTypes: nil},
	}}
	Types["AtrulePrelude"] = &TypeDef{Name: "AtrulePrelude", WalkContext: "atrulePrelude", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<any>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: nil},
	}}
	Types["AttributeSelector"] = &TypeDef{Name: "AttributeSelector", WalkContext: "", Fields: []FieldDef{
		{Name: "name", Declared: []string{"Identifier"}, String: false, Boolean: false, Nullable: false, NodeTypes: []string{"Identifier"}, IsList: false, ListTypes: nil},
		{Name: "matcher", Declared: []string{"string", "null"}, String: true, Boolean: false, Nullable: true, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "value", Declared: []string{"String", "Identifier", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: []string{"String", "Identifier"}, IsList: false, ListTypes: nil},
		{Name: "flags", Declared: []string{"string", "null"}, String: true, Boolean: false, Nullable: true, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Block"] = &TypeDef{Name: "Block", WalkContext: "block", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<Atrule | Rule | Declaration>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: []string{"Atrule", "Rule", "Declaration"}},
	}}
	Types["Brackets"] = &TypeDef{Name: "Brackets", WalkContext: "", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<any>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: nil},
	}}
	Types["CDC"] = &TypeDef{Name: "CDC", WalkContext: "", Fields: []FieldDef{}}
	Types["CDO"] = &TypeDef{Name: "CDO", WalkContext: "", Fields: []FieldDef{}}
	Types["ClassSelector"] = &TypeDef{Name: "ClassSelector", WalkContext: "", Fields: []FieldDef{
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Combinator"] = &TypeDef{Name: "Combinator", WalkContext: "", Fields: []FieldDef{
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Comment"] = &TypeDef{Name: "Comment", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Condition"] = &TypeDef{Name: "Condition", WalkContext: "", Fields: []FieldDef{
		{Name: "kind", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "children", Declared: []string{"List<Identifier | Feature | FeatureFunction | FeatureRange | SupportsDeclaration>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: []string{"Identifier", "Feature", "FeatureFunction", "FeatureRange", "SupportsDeclaration"}},
	}}
	Types["Declaration"] = &TypeDef{Name: "Declaration", WalkContext: "declaration", Fields: []FieldDef{
		{Name: "important", Declared: []string{"boolean", "string"}, String: true, Boolean: true, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "property", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "value", Declared: []string{"Value", "Raw"}, String: false, Boolean: false, Nullable: false, NodeTypes: []string{"Value", "Raw"}, IsList: false, ListTypes: nil},
	}}
	Types["DeclarationList"] = &TypeDef{Name: "DeclarationList", WalkContext: "", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<Declaration | Atrule | Rule>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: []string{"Declaration", "Atrule", "Rule"}},
	}}
	Types["Dimension"] = &TypeDef{Name: "Dimension", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "unit", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Feature"] = &TypeDef{Name: "Feature", WalkContext: "", Fields: []FieldDef{
		{Name: "kind", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "value", Declared: []string{"Identifier", "Number", "Dimension", "Ratio", "Function", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: []string{"Identifier", "Number", "Dimension", "Ratio", "Function"}, IsList: false, ListTypes: nil},
	}}
	Types["FeatureFunction"] = &TypeDef{Name: "FeatureFunction", WalkContext: "", Fields: []FieldDef{
		{Name: "kind", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "feature", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "value", Declared: []string{"Declaration", "Selector"}, String: false, Boolean: false, Nullable: false, NodeTypes: []string{"Declaration", "Selector"}, IsList: false, ListTypes: nil},
	}}
	Types["FeatureRange"] = &TypeDef{Name: "FeatureRange", WalkContext: "", Fields: []FieldDef{
		{Name: "kind", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "left", Declared: []string{"Identifier", "Number", "Dimension", "Ratio", "Function"}, String: false, Boolean: false, Nullable: false, NodeTypes: []string{"Identifier", "Number", "Dimension", "Ratio", "Function"}, IsList: false, ListTypes: nil},
		{Name: "leftComparison", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "middle", Declared: []string{"Identifier", "Number", "Dimension", "Ratio", "Function"}, String: false, Boolean: false, Nullable: false, NodeTypes: []string{"Identifier", "Number", "Dimension", "Ratio", "Function"}, IsList: false, ListTypes: nil},
		{Name: "rightComparison", Declared: []string{"string", "null"}, String: true, Boolean: false, Nullable: true, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "right", Declared: []string{"Identifier", "Number", "Dimension", "Ratio", "Function", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: []string{"Identifier", "Number", "Dimension", "Ratio", "Function"}, IsList: false, ListTypes: nil},
	}}
	Types["Function"] = &TypeDef{Name: "Function", WalkContext: "function", Fields: []FieldDef{
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "children", Declared: []string{"List<any>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: nil},
	}}
	Types["GeneralEnclosed"] = &TypeDef{Name: "GeneralEnclosed", WalkContext: "", Fields: []FieldDef{
		{Name: "kind", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "function", Declared: []string{"string", "null"}, String: true, Boolean: false, Nullable: true, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "children", Declared: []string{"List<any>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: nil},
	}}
	Types["Hash"] = &TypeDef{Name: "Hash", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["IdSelector"] = &TypeDef{Name: "IdSelector", WalkContext: "", Fields: []FieldDef{
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Identifier"] = &TypeDef{Name: "Identifier", WalkContext: "", Fields: []FieldDef{
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Layer"] = &TypeDef{Name: "Layer", WalkContext: "", Fields: []FieldDef{
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["LayerList"] = &TypeDef{Name: "LayerList", WalkContext: "", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<Layer>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: []string{"Layer"}},
	}}
	Types["MediaQuery"] = &TypeDef{Name: "MediaQuery", WalkContext: "", Fields: []FieldDef{
		{Name: "modifier", Declared: []string{"string", "null"}, String: true, Boolean: false, Nullable: true, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "mediaType", Declared: []string{"string", "null"}, String: true, Boolean: false, Nullable: true, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "condition", Declared: []string{"Condition", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: []string{"Condition"}, IsList: false, ListTypes: nil},
	}}
	Types["MediaQueryList"] = &TypeDef{Name: "MediaQueryList", WalkContext: "", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<MediaQuery>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: []string{"MediaQuery"}},
	}}
	Types["NestingSelector"] = &TypeDef{Name: "NestingSelector", WalkContext: "", Fields: []FieldDef{}}
	Types["Nth"] = &TypeDef{Name: "Nth", WalkContext: "", Fields: []FieldDef{
		{Name: "nth", Declared: []string{"AnPlusB", "Identifier"}, String: false, Boolean: false, Nullable: false, NodeTypes: []string{"AnPlusB", "Identifier"}, IsList: false, ListTypes: nil},
		{Name: "selector", Declared: []string{"SelectorList", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: []string{"SelectorList"}, IsList: false, ListTypes: nil},
	}}
	Types["Number"] = &TypeDef{Name: "Number", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Operator"] = &TypeDef{Name: "Operator", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Parentheses"] = &TypeDef{Name: "Parentheses", WalkContext: "", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<any>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: nil},
	}}
	Types["Percentage"] = &TypeDef{Name: "Percentage", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["PseudoClassSelector"] = &TypeDef{Name: "PseudoClassSelector", WalkContext: "function", Fields: []FieldDef{
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "children", Declared: []string{"List<Raw>", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: nil, IsList: true, ListTypes: []string{"Raw"}},
	}}
	Types["PseudoElementSelector"] = &TypeDef{Name: "PseudoElementSelector", WalkContext: "function", Fields: []FieldDef{
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
		{Name: "children", Declared: []string{"List<Raw>", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: nil, IsList: true, ListTypes: []string{"Raw"}},
	}}
	Types["Ratio"] = &TypeDef{Name: "Ratio", WalkContext: "", Fields: []FieldDef{
		{Name: "left", Declared: []string{"Number", "Function"}, String: false, Boolean: false, Nullable: false, NodeTypes: []string{"Number", "Function"}, IsList: false, ListTypes: nil},
		{Name: "right", Declared: []string{"Number", "Function", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: []string{"Number", "Function"}, IsList: false, ListTypes: nil},
	}}
	Types["Raw"] = &TypeDef{Name: "Raw", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Rule"] = &TypeDef{Name: "Rule", WalkContext: "rule", Fields: []FieldDef{
		{Name: "prelude", Declared: []string{"SelectorList", "Raw"}, String: false, Boolean: false, Nullable: false, NodeTypes: []string{"SelectorList", "Raw"}, IsList: false, ListTypes: nil},
		{Name: "block", Declared: []string{"Block"}, String: false, Boolean: false, Nullable: false, NodeTypes: []string{"Block"}, IsList: false, ListTypes: nil},
	}}
	Types["Scope"] = &TypeDef{Name: "Scope", WalkContext: "", Fields: []FieldDef{
		{Name: "root", Declared: []string{"SelectorList", "Raw", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: []string{"SelectorList", "Raw"}, IsList: false, ListTypes: nil},
		{Name: "limit", Declared: []string{"SelectorList", "Raw", "null"}, String: false, Boolean: false, Nullable: true, NodeTypes: []string{"SelectorList", "Raw"}, IsList: false, ListTypes: nil},
	}}
	Types["Selector"] = &TypeDef{Name: "Selector", WalkContext: "", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<TypeSelector | IdSelector | ClassSelector | AttributeSelector | PseudoClassSelector | PseudoElementSelector | Combinator>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: []string{"TypeSelector", "IdSelector", "ClassSelector", "AttributeSelector", "PseudoClassSelector", "PseudoElementSelector", "Combinator"}},
	}}
	Types["SelectorList"] = &TypeDef{Name: "SelectorList", WalkContext: "selector", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<Selector | Raw>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: []string{"Selector", "Raw"}},
	}}
	Types["String"] = &TypeDef{Name: "String", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["StyleSheet"] = &TypeDef{Name: "StyleSheet", WalkContext: "stylesheet", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<Comment | CDO | CDC | Atrule | Rule | Raw>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: []string{"Comment", "CDO", "CDC", "Atrule", "Rule", "Raw"}},
	}}
	Types["SupportsDeclaration"] = &TypeDef{Name: "SupportsDeclaration", WalkContext: "", Fields: []FieldDef{
		{Name: "declaration", Declared: []string{"Declaration"}, String: false, Boolean: false, Nullable: false, NodeTypes: []string{"Declaration"}, IsList: false, ListTypes: nil},
	}}
	Types["TypeSelector"] = &TypeDef{Name: "TypeSelector", WalkContext: "", Fields: []FieldDef{
		{Name: "name", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["UnicodeRange"] = &TypeDef{Name: "UnicodeRange", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Url"] = &TypeDef{Name: "Url", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
	Types["Value"] = &TypeDef{Name: "Value", WalkContext: "", Fields: []FieldDef{
		{Name: "children", Declared: []string{"List<any>"}, String: false, Boolean: false, Nullable: false, NodeTypes: nil, IsList: true, ListTypes: nil},
	}}
	Types["WhiteSpace"] = &TypeDef{Name: "WhiteSpace", WalkContext: "", Fields: []FieldDef{
		{Name: "value", Declared: []string{"string"}, String: true, Boolean: false, Nullable: false, NodeTypes: nil, IsList: false, ListTypes: nil},
	}}
}
