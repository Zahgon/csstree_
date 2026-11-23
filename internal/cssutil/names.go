package cssutil

import "github.com/csstree/csstree/internal/jsutil"

// KeywordDescriptor is what a keyword is made of: its lower cased name, the vendor prefix it
// carries, the name without that prefix, and whether it is a custom property name.
type KeywordDescriptor struct {
	Basename jsutil.Str
	Name     jsutil.Str
	Prefix   jsutil.Str
	Vendor   jsutil.Str
	Custom   bool
}

// PropertyDescriptor is what a property name is made of: the hack it begins with, the vendor
// prefix, the two together, the name without the hack and the name without either.
type PropertyDescriptor struct {
	Basename jsutil.Str
	Name     jsutil.Str
	Hack     jsutil.Str
	Vendor   jsutil.Str
	Prefix   jsutil.Str
	Custom   bool
}

// IsCustomProperty reports whether the name, read from the offset on, begins with two hyphens.
func IsCustomProperty(str jsutil.Str, offset int) bool {
	return len(str)-offset >= 2 &&
		str.CharCodeAt(offset) == hyphenMinus &&
		str.CharCodeAt(offset+1) == hyphenMinus
}

// VendorPrefix is the vendor prefix the name carries from the offset on, or an empty string. A
// prefix is at least three code units, begins with one hyphen that is not followed by another,
// and ends at the next hyphen.
func VendorPrefix(str jsutil.Str, offset int) jsutil.Str {
	if len(str)-offset >= 3 {
		if str.CharCodeAt(offset) == hyphenMinus && str.CharCodeAt(offset+1) != hyphenMinus {
			secondDashIndex := str.IndexOfFrom(jsutil.Str{hyphenMinus}, offset+2)

			if secondDashIndex != -1 {
				return str.Substring(offset, secondDashIndex+1)
			}
		}
	}

	return jsutil.Str{}
}

// Keyword reads a keyword into its descriptor.
func Keyword(keyword jsutil.Str) KeywordDescriptor {
	name := keyword.ToLower()
	custom := IsCustomProperty(name, 0)
	vendor := jsutil.Str{}
	if !custom {
		vendor = VendorPrefix(name, 0)
	}

	return KeywordDescriptor{
		Basename: name.Slice(len(vendor)),
		Name:     name,
		Prefix:   vendor,
		Vendor:   vendor,
		Custom:   custom,
	}
}

// Property reads a property name into its descriptor, keeping the hack characters a stylesheet
// may write in front of it.
func Property(property jsutil.Str) PropertyDescriptor {
	name := property
	hack := jsutil.Str{}

	if len(property) > 0 {
		switch property[0] {
		case '/':
			if property.CharCodeAt(1) == '/' {
				hack = jsutil.FromString("//")
			} else {
				hack = jsutil.FromString("/")
			}
		case '_', '*', '$', '#', '+', '&':
			hack = property.Substring(0, 1)
		}
	}

	custom := IsCustomProperty(name, len(hack))

	// re-use result when possible (the same as for lower case)
	if !custom {
		name = name.ToLower()
	}

	vendor := jsutil.Str{}
	if !custom {
		vendor = VendorPrefix(name, len(hack))
	}
	prefix := name.Substring(0, len(hack)+len(vendor))

	return PropertyDescriptor{
		Basename: name.Slice(len(prefix)),
		Name:     name.Slice(len(hack)),
		Hack:     hack,
		Vendor:   vendor,
		Prefix:   prefix,
		Custom:   custom,
	}
}
