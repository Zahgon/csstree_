// Package dict resolves the CSS syntax dictionary: the value definition syntax of every property,
// type and at-rule the lexer knows. It takes the raw CSS data of the MDN data set and the
// repository's own patch, and applies the patch the way the original does, keeping the order the
// entries appear in, because that order is the order the dictionary is written out in.
package dict

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/csstree/csstree/internal/jsutil"
)

// Atrule is one at-rule of the dictionary: the definition of its prelude, which may be absent,
// and the definitions of its descriptors, which may be absent as a whole.
type Atrule struct {
	Prelude     jsutil.Str
	HasPrelude  bool
	Descriptors *jsutil.Object // descriptor name -> definition text; nil when the at-rule has none
}

// AtruleMap holds at-rules in the order they were added.
type AtruleMap struct {
	keys   []jsutil.Str
	values []*Atrule
}

// NewAtruleMap returns an empty map.
func NewAtruleMap() *AtruleMap {
	return &AtruleMap{}
}

// Set adds an at-rule, keeping the place a name already there has.
func (m *AtruleMap) Set(key jsutil.Str, value *Atrule) {
	for i, k := range m.keys {
		if k.Equal(key) {
			m.values[i] = value
			return
		}
	}
	m.keys = append(m.keys, key)
	m.values = append(m.values, value)
}

// Get returns the at-rule of a name.
func (m *AtruleMap) Get(key jsutil.Str) (*Atrule, bool) {
	for i, k := range m.keys {
		if k.Equal(key) {
			return m.values[i], true
		}
	}
	return nil, false
}

// Has reports whether the name is there.
func (m *AtruleMap) Has(key jsutil.Str) bool {
	_, ok := m.Get(key)
	return ok
}

// Keys are the names in their order.
func (m *AtruleMap) Keys() []jsutil.Str {
	return m.keys
}

// Dict is the resolved dictionary.
type Dict struct {
	Types      *jsutil.Object // type name -> definition text
	Atrules    *AtruleMap     // at-rule name (without the "@") -> its prelude and descriptors
	Properties *jsutil.Object // property name -> definition text
}

// Build resolves the dictionary from the four data documents: the MDN at-rules, properties and
// syntaxes, and the repository patch.
func Build(mdnAtrules, mdnProperties, mdnSyntaxes, patch jsutil.Value) (*Dict, error) {
	atrulesData, ok := mdnAtrules.(*jsutil.Object)
	if !ok {
		return nil, fmt.Errorf("the at-rules data is not an object")
	}
	propertiesData, ok := mdnProperties.(*jsutil.Object)
	if !ok {
		return nil, fmt.Errorf("the properties data is not an object")
	}
	syntaxesData, ok := mdnSyntaxes.(*jsutil.Object)
	if !ok {
		return nil, fmt.Errorf("the syntaxes data is not an object")
	}
	patchData, ok := patch.(*jsutil.Object)
	if !ok {
		return nil, fmt.Errorf("the patch is not an object")
	}

	preprocessed, err := preprocessAtrules(atrulesData)
	if err != nil {
		return nil, err
	}
	atrulePatch := objectAt(patchData, "atrules")
	typePatch := objectAt(patchData, "types")
	propertyPatch := objectAt(patchData, "properties")

	return &Dict{
		Types:      patchDictionary(syntaxesData, typePatch),
		Atrules:    patchAtrules(preprocessed, atrulePatch),
		Properties: patchDictionary(propertiesData, propertyPatch),
	}, nil
}

func objectAt(object *jsutil.Object, key string) *jsutil.Object {
	value, ok := object.Get(jsutil.FromString(key))
	if !ok {
		return jsutil.NewObject()
	}
	nested, ok := value.(*jsutil.Object)
	if !ok {
		return jsutil.NewObject()
	}
	return nested
}

func stringAt(object *jsutil.Object, key string) (jsutil.Str, bool) {
	value, ok := object.Get(jsutil.FromString(key))
	if !ok {
		return nil, false
	}
	text, ok := value.(jsutil.JSONString)
	if !ok {
		return nil, false
	}
	return jsutil.Str(text), true
}

// The patch marks a definition that extends the one it patches rather than replacing it.
var extendSyntax = regexp.MustCompile(`^\s*\|\s*`)

// The prelude of an at-rule is what is left of its full definition once the block is taken out.
var atruleBlock = regexp.MustCompile(`(?s)\{.+\}`)
var atrulePrelude = regexp.MustCompile(`^@\S+\s+([^;{]*)`)

// preprocessAtrules turns the MDN at-rules into the shape the patch is applied to: the name
// without its "@", the prelude definition taken out of the full definition, and the descriptors
// reduced to their definition text.
func preprocessAtrules(data *jsutil.Object) (*AtruleMap, error) {
	result := NewAtruleMap()
	for _, key := range data.Keys() {
		value, _ := data.Get(key)
		atrule, ok := value.(*jsutil.Object)
		if !ok {
			continue
		}
		var descriptors *jsutil.Object
		if nested, ok := atrule.Get(jsutil.FromString("descriptors")); ok {
			if fields, ok := nested.(*jsutil.Object); ok {
				descriptors = jsutil.NewObject()
				for _, name := range fields.Keys() {
					descriptor, _ := fields.Get(name)
					if object, ok := descriptor.(*jsutil.Object); ok {
						if syntax, ok := stringAt(object, "syntax"); ok {
							descriptors.Set(name, jsutil.JSONString(syntax))
							continue
						}
					}
					descriptors.Set(name, jsutil.Null{})
				}
			}
		}
		syntax, ok := stringAt(atrule, "syntax")
		if !ok {
			return nil, fmt.Errorf("the at-rule %q has no syntax", key.String())
		}
		text := atruleBlock.ReplaceAllString(strings.TrimSpace(syntax.String()), "")
		match := atrulePrelude.FindStringSubmatch(text)
		if match == nil {
			return nil, fmt.Errorf("the at-rule %q has a definition no prelude can be read from", key.String())
		}
		prelude := strings.TrimSpace(match[1])
		entry := &Atrule{Descriptors: descriptors}
		if prelude != "" {
			entry.Prelude = jsutil.FromString(prelude)
			entry.HasPrelude = true
		}
		result.Set(key.Slice(1), entry)
	}
	return result, nil
}

// patchDictionary copies the definitions of a dictionary and applies a patch to them: a patched
// definition either replaces the one that was there or, when it begins with a bar, is added to
// it, and a patch entry with no definition removes the entry.
func patchDictionary(dict, patchDict *jsutil.Object) *jsutil.Object {
	result := jsutil.NewObject()
	for _, key := range dict.Keys() {
		value, _ := dict.Get(key)
		switch entry := value.(type) {
		case *jsutil.Object:
			if syntax, ok := stringAt(entry, "syntax"); ok {
				result.Set(key, jsutil.JSONString(syntax))
			} else {
				result.Set(key, entry)
			}
		case jsutil.JSONString:
			if len(entry) > 0 {
				result.Set(key, entry)
			}
		case jsutil.Null:
			// A missing definition is left out, as a falsy value is in the original.
		default:
			if entry != nil {
				result.Set(key, entry)
			}
		}
	}
	for _, key := range patchDict.Keys() {
		value, _ := patchDict.Get(key)
		entry, _ := value.(*jsutil.Object)
		var syntax jsutil.Str
		hasSyntax := false
		if entry != nil {
			if text, ok := stringAt(entry, "syntax"); ok && len(text) > 0 {
				syntax, hasSyntax = text, true
			}
		}
		if dict.Has(key) {
			if hasSyntax {
				if extendSyntax.MatchString(syntax.String()) {
					if previous, ok := result.Get(key); ok {
						if text, ok := previous.(jsutil.JSONString); ok {
							result.Set(key, jsutil.JSONString(jsutil.FromString(
								jsutil.Str(text).String()+" "+strings.TrimSpace(syntax.String()))))
							continue
						}
					}
				}
				result.Set(key, jsutil.JSONString(syntax))
			} else {
				result.Delete(key)
			}
		} else if hasSyntax {
			result.Set(key, jsutil.JSONString(jsutil.FromString(extendSyntax.ReplaceAllString(syntax.String(), ""))))
		}
	}
	return result
}

// patchAtrules applies the at-rule part of the patch: an at-rule the patch sets to nothing is
// dropped, a prelude the patch names replaces the one that was read from the data, and the
// descriptors are patched as a dictionary of their own.
func patchAtrules(dict *AtruleMap, patchDict *jsutil.Object) *AtruleMap {
	result := NewAtruleMap()
	for _, key := range dict.Keys() {
		patchValue, patched := patchDict.Get(key)
		if patched {
			if _, isNull := patchValue.(jsutil.Null); isNull {
				continue
			}
		}
		atrulePatch, _ := patchValue.(*jsutil.Object)
		original, _ := dict.Get(key)

		entry := &Atrule{}
		if atrulePatch != nil && atrulePatch.Has(jsutil.FromString("prelude")) {
			if text, ok := stringAt(atrulePatch, "prelude"); ok {
				entry.Prelude, entry.HasPrelude = text, true
			}
		} else {
			entry.Prelude, entry.HasPrelude = original.Prelude, original.HasPrelude
		}
		descriptors := original.Descriptors
		if descriptors == nil {
			descriptors = jsutil.NewObject()
		}
		var patchDescriptors *jsutil.Object
		if atrulePatch != nil {
			patchDescriptors = preprocessPatchDescriptors(objectAt(atrulePatch, "descriptors"))
		} else {
			patchDescriptors = jsutil.NewObject()
		}
		entry.Descriptors = patchDictionary(descriptors, patchDescriptors)
		result.Set(key, entry)
	}
	for _, key := range patchDict.Keys() {
		value, _ := patchDict.Get(key)
		atrulePatch, ok := value.(*jsutil.Object)
		if !ok || dict.Has(key) {
			continue
		}
		entry := &Atrule{}
		if text, ok := stringAt(atrulePatch, "prelude"); ok && len(text) > 0 {
			entry.Prelude, entry.HasPrelude = text, true
		}
		if atrulePatch.Has(jsutil.FromString("descriptors")) {
			if _, isNull := mustGet(atrulePatch, "descriptors").(jsutil.Null); !isNull {
				entry.Descriptors = patchDictionary(jsutil.NewObject(),
					preprocessPatchDescriptors(objectAt(atrulePatch, "descriptors")))
			}
		}
		result.Set(key, entry)
	}
	return result
}

func mustGet(object *jsutil.Object, key string) jsutil.Value {
	value, _ := object.Get(jsutil.FromString(key))
	return value
}

// preprocessPatchDescriptors lets a descriptor of the patch be written either as its definition
// text or as an object holding it.
func preprocessPatchDescriptors(declarations *jsutil.Object) *jsutil.Object {
	result := jsutil.NewObject()
	for _, key := range declarations.Keys() {
		value, _ := declarations.Get(key)
		if text, ok := value.(jsutil.JSONString); ok {
			result.Set(key, jsutil.NewObject().SetString("syntax", text))
			continue
		}
		result.Set(key, value)
	}
	return result
}
