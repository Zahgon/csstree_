package lexer

import (
	"math"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/jsutil"
)

// StructureWarning is one thing wrong with a tree: the node it was found on and what is wrong.
type StructureWarning struct {
	NodeType string
	HasType  bool
	Message  string
}

// listValue marks a value that was an array before it was read in, which the structure check
// expects a list of children to be.
type listValue struct {
	items []any
}

// ToListTree turns a plain value into the tree the structure check walks: every array becomes a
// list and nothing else is changed, so a tree that does not match any node type is checked as it
// was given.
func ToListTree(value jsutil.Value) any {
	switch v := value.(type) {
	case jsutil.Array:
		items := make([]any, len(v))
		for i, item := range v {
			items[i] = ToListTree(item)
		}
		return &listValue{items: items}
	case *jsutil.Object:
		out := map[string]any{}
		order := []string{}
		for _, key := range v.Keys() {
			field, _ := v.Get(key)
			name := key.String()
			out[name] = ToListTree(field)
			order = append(order, name)
		}
		return &objectValue{fields: out, order: order}
	}
	return value
}

// objectValue is an object of the tree, keeping the order its keys were written in.
type objectValue struct {
	fields map[string]any
	order  []string
}

func (o *objectValue) typeName() (string, bool) {
	value, ok := o.fields["type"]
	if !ok {
		return "", false
	}
	text, ok := value.(jsutil.JSONString)
	if !ok {
		return "", false
	}
	return jsutil.Str(text).String(), true
}

// CheckStructure walks a tree and reports every field that is missing, unknown or holds something
// its node type does not allow.
func CheckStructure(tree any) []StructureWarning {
	warnings := []StructureWarning{}

	var walk func(node any)
	walk = func(node any) {
		object, ok := node.(*objectValue)
		if !ok {
			return
		}

		nodeType, hasType := object.typeName()
		def, known := ast.Types[nodeType]

		if !known {
			warnings = append(warnings, StructureWarning{NodeType: nodeType, HasType: hasType,
				Message: "Unknown node type `" + nodeType + "`"})
			return
		}

		checkNode(object, nodeType, def, &warnings)

		for _, field := range def.Fields {
			value, present := object.fields[field.Name]
			if !present {
				continue
			}

			switch held := value.(type) {
			case *objectValue:
				walk(held)
			case *listValue:
				for _, item := range held.items {
					walk(item)
				}
			}
		}
	}

	walk(tree)

	if len(warnings) == 0 {
		return nil
	}

	return warnings
}

// checkNode reports what is wrong with one node, in the order the original reports it: first
// every key the node has, then every field the node type declares and the node does not.
func checkNode(object *objectValue, nodeType string, def *ast.TypeDef, warnings *[]StructureWarning) {
	hasType := true
	if _, ok := object.typeName(); !ok {
		hasType = false
	}

	fields := map[string]*ast.FieldDef{}
	for i := range def.Fields {
		fields[def.Fields[i].Name] = &def.Fields[i]
	}

	warn := func(message string) {
		*warnings = append(*warnings, StructureWarning{NodeType: nodeType, HasType: hasType, Message: message})
	}

	for _, key := range object.order {
		value := object.fields[key]
		valid := true

		switch {
		case key == "type":
			if text, ok := value.(jsutil.JSONString); !ok || jsutil.Str(text).String() != def.Name {
				warn("Wrong node type `" + nodeType + "`, expected `" + def.Name + "`")
			}
			continue

		case key == "loc":
			if _, isNull := value.(jsutil.Null); isNull {
				continue
			}

			if loc, isObject := value.(*objectValue); isObject {
				suffix := ""
				switch {
				case !isString(loc.fields["source"]):
					suffix = ".source"
				case !isValidLocation(loc.fields["start"]):
					suffix = ".start"
				case !isValidLocation(loc.fields["end"]):
					suffix = ".end"
				default:
					continue
				}

				warn("Bad value for `" + nodeType + ".loc" + suffix + "`")
				continue
			}

			warn("Bad value for `" + nodeType + ".loc`")
			continue

		default:
			field, declared := fields[key]
			if !declared {
				warn("Unknown field `" + key + "` for " + nodeType + " node type")
				continue
			}

			valid = fieldHolds(field, value)
		}

		if !valid {
			warn("Bad value for `" + nodeType + "." + key + "`")
		}
	}

	for _, field := range def.Fields {
		if _, present := object.fields[field.Name]; !present {
			warn("Field `" + nodeType + "." + field.Name + "` is missed")
		}
	}

	if _, present := object.fields["loc"]; !present {
		*warnings = append(*warnings, StructureWarning{NodeType: nodeType, HasType: hasType,
			Message: "Field `" + nodeType + ".loc` is missed"})
	}
}

// fieldHolds reports whether a value is one of the things a field may hold.
func fieldHolds(field *ast.FieldDef, value any) bool {
	if field.String && isString(value) {
		return true
	}

	if field.Boolean {
		if _, ok := value.(jsutil.Bool); ok {
			return true
		}
	}

	if field.Nullable {
		if _, ok := value.(jsutil.Null); ok {
			return true
		}
	}

	if len(field.NodeTypes) > 0 {
		if object, ok := value.(*objectValue); ok {
			name, _ := object.typeName()
			for _, allowed := range field.NodeTypes {
				if name == allowed {
					return true
				}
			}
		}
	}

	if field.IsList {
		if _, ok := value.(*listValue); ok {
			return true
		}
	}

	return false
}

func isString(value any) bool {
	_, ok := value.(jsutil.JSONString)
	return ok
}

// isValidLocation reports whether a point of a location holds three whole numbers that are not
// below zero.
func isValidLocation(value any) bool {
	point, ok := value.(*objectValue)
	if !ok {
		return false
	}

	for _, key := range []string{"offset", "line", "column"} {
		if !isValidNumber(point.fields[key]) {
			return false
		}
	}

	return true
}

func isValidNumber(value any) bool {
	switch n := value.(type) {
	case jsutil.Int:
		return int(n) >= 0
	case jsutil.Num:
		f := float64(n)
		return !math.IsInf(f, 0) && !math.IsNaN(f) && math.Floor(f) == f && f >= 0
	}
	return false
}
