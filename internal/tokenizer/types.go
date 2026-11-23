// Package tokenizer turns CSS text into the token stream the rest of the module reads: the token
// types of CSS Syntax Module Level 3, the character predicates and scanning helpers the algorithms
// are written against, the token stream with its balance table, and the offset to line and column
// map. Every offset is an index into a jsutil.Str, that is a UTF-16 code unit index, because that
// is what the original JavaScript implementation counts.
package tokenizer

// The token types of CSS Syntax Module Level 3 (https://www.w3.org/TR/css-syntax-3/).
const (
	EOF                = 0  // <EOF-token>
	Ident              = 1  // <ident-token>
	Function           = 2  // <function-token>
	AtKeyword          = 3  // <at-keyword-token>
	Hash               = 4  // <hash-token>
	String             = 5  // <string-token>
	BadString          = 6  // <bad-string-token>
	Url                = 7  // <url-token>
	BadUrl             = 8  // <bad-url-token>
	Delim              = 9  // <delim-token>
	Number             = 10 // <number-token>
	Percentage         = 11 // <percentage-token>
	Dimension          = 12 // <dimension-token>
	WhiteSpace         = 13 // <whitespace-token>
	CDO                = 14 // <CDO-token>
	CDC                = 15 // <CDC-token>
	Colon              = 16 // <colon-token>     :
	Semicolon          = 17 // <semicolon-token> ;
	Comma              = 18 // <comma-token>     ,
	LeftSquareBracket  = 19 // <[-token>
	RightSquareBracket = 20 // <]-token>
	LeftParenthesis    = 21 // <(-token>
	RightParenthesis   = 22 // <)-token>
	LeftCurlyBracket   = 23 // <{-token>
	RightCurlyBracket  = 24 // <}-token>
	Comment            = 25
)

// Names holds the name of every token type, indexed by the type.
var Names = []string{
	"EOF-token",
	"ident-token",
	"function-token",
	"at-keyword-token",
	"hash-token",
	"string-token",
	"bad-string-token",
	"url-token",
	"bad-url-token",
	"delim-token",
	"number-token",
	"percentage-token",
	"dimension-token",
	"whitespace-token",
	"CDO-token",
	"CDC-token",
	"colon-token",
	"semicolon-token",
	"comma-token",
	"[-token",
	"]-token",
	"(-token",
	")-token",
	"{-token",
	"}-token",
	"comment-token",
}
