package tokenizer

import "github.com/csstree/csstree/internal/jsutil"

const (
	offsetMask      = 0x00FFFFFF
	typeShift       = 24
	blockOpenToken  = 1
	blockCloseToken = 2
)

// 32 entries of memory ought to be enough for anyone (any number of token types).
var balancePair = func() [32]int {
	var pair [32]int
	pair[Function] = RightParenthesis
	pair[LeftParenthesis] = RightParenthesis
	pair[LeftSquareBracket] = RightSquareBracket
	pair[LeftCurlyBracket] = RightCurlyBracket
	return pair
}()

var blockTokens = func() [32]int {
	var tokens [32]int
	tokens[Function] = blockOpenToken
	tokens[LeftParenthesis] = blockOpenToken
	tokens[LeftSquareBracket] = blockOpenToken
	tokens[LeftCurlyBracket] = blockOpenToken
	tokens[RightParenthesis] = blockCloseToken
	tokens[RightSquareBracket] = blockCloseToken
	tokens[RightCurlyBracket] = blockCloseToken
	return tokens
}()

// balancePairOf and blockTokenOf read the tables the way JavaScript reads an array: a type outside
// the table reads as nothing, which matches none of the comparisons the callers make.
func balancePairOf(tokenType int) int {
	if tokenType < 0 || tokenType >= len(balancePair) {
		return 0
	}
	return balancePair[tokenType]
}

func blockTokenOf(tokenType int) int {
	if tokenType < 0 || tokenType >= len(blockTokens) {
		return 0
	}
	return blockTokens[tokenType]
}

func boundIndex(index, min, max int) int {
	if index < min {
		return min
	}
	if index > max {
		return max
	}
	return index
}

const minBufferSize = 16 * 1024

// adoptBuffer keeps a buffer that is large enough and allocates a bigger one otherwise, as
// adopt-buffer.js does.
func adoptBuffer(buffer []uint32, size int) []uint32 {
	if buffer == nil || len(buffer) < size {
		n := size + 1024
		if n < minBufferSize {
			n = minBufferSize
		}
		return make([]uint32, n)
	}

	return buffer
}

// TokenStream walks the tokens of a source. Beside the current token it keeps, for every token,
// its end offset and type packed into one word and its balance: the index of the token that closes
// the block the token is in, or that the token itself opens.
type TokenStream struct {
	Source          jsutil.Str
	FirstCharOffset int
	TokenType       int
	TokenStart      int
	TokenEnd        int
	TokenIndex      int
	TokenCount      int
	Balance         []uint32
	EOF             bool

	offsetAndType []uint32
}

// NewTokenStream creates a stream over source, tokenized by tokenize, positioned on the first
// token.
func NewTokenStream(source jsutil.Str, tokenize func(jsutil.Str, func(int, int, int))) *TokenStream {
	s := &TokenStream{}
	s.SetSource(source, tokenize)
	return s
}

// Reset puts the stream back before its first token.
func (s *TokenStream) Reset() {
	s.EOF = false
	s.TokenIndex = -1
	s.TokenType = 0
	s.TokenStart = s.FirstCharOffset
	s.TokenEnd = s.FirstCharOffset
}

// SetSource tokenizes source, builds the offset, type and balance tables and moves to the first
// token. A nil tokenize leaves the stream with no tokens, as the original's default does.
func (s *TokenStream) SetSource(source jsutil.Str, tokenize func(jsutil.Str, func(int, int, int))) {
	if tokenize == nil {
		tokenize = func(jsutil.Str, func(int, int, int)) {}
	}

	sourceLength := len(source)
	offsetAndType := adoptBuffer(s.offsetAndType, sourceLength+1) // +1 because of eof-token
	balance := adoptBuffer(s.Balance, sourceLength+1)
	tokenCount := 0
	firstCharOffset := -1
	balanceCloseType := 0
	balanceStart := sourceLength

	// capture buffers
	s.offsetAndType = nil
	s.Balance = nil
	for i := range balance {
		balance[i] = 0
	}

	tokenize(source, func(tokenType, start, end int) {
		index := tokenCount
		tokenCount++

		// type & offset
		offsetAndType[index] = uint32(tokenType<<typeShift | end)

		if firstCharOffset == -1 {
			firstCharOffset = start
		}

		// balance
		balance[index] = uint32(balanceStart)

		if tokenType == balanceCloseType {
			prevBalanceStart := int(balance[balanceStart])

			// set reference to balance end for a block opener
			balance[balanceStart] = uint32(index)

			// pop state
			balanceStart = prevBalanceStart
			balanceCloseType = balancePairOf(int(int32(offsetAndType[prevBalanceStart]) >> typeShift))
		} else if s.IsBlockOpenerTokenType(tokenType) { // check for FunctionToken, <(-token>, <[-token> and <{-token>
			// push state
			balanceStart = index
			balanceCloseType = balancePairOf(tokenType)
		}
	})

	// finalize buffers
	offsetAndType[tokenCount] = uint32(EOF<<typeShift | sourceLength) // <EOF-token>
	balance[tokenCount] = uint32(tokenCount)                          // prevents false positive balance match with any token

	// reverse references from balance start to end
	// tokens
	//   token:   a ( [ b c ] d e ) {
	//   index:   0 1 2 3 4 5 6 7 8 9
	// before
	//   balance: 0 8 5 2 2 2 1 1 1 0
	//            - > > < < < < < < -
	// after
	//   balance: 9 8 5 5 5 2 8 8 1 9
	//            > > > > > < > > < >
	for i := 0; i < tokenCount; i++ {
		balanceStart := int(balance[i])

		if balanceStart <= i {
			balanceEnd := int(balance[balanceStart])

			if balanceEnd != i {
				balance[i] = uint32(balanceEnd)
			}
		} else if balanceStart > tokenCount {
			balance[i] = uint32(tokenCount)
		}
	}

	s.Source = source
	if firstCharOffset == -1 {
		s.FirstCharOffset = 0
	} else {
		s.FirstCharOffset = firstCharOffset
	}
	s.TokenCount = tokenCount
	s.offsetAndType = offsetAndType
	s.Balance = balance

	s.Reset()
	s.Next()
}

// offsetAt and typeAt read a packed word, giving the 0 a JavaScript array read gives outside its
// bounds.
func (s *TokenStream) offsetAt(index int) int {
	if index < 0 || index >= len(s.offsetAndType) {
		return 0
	}
	return int(s.offsetAndType[index] & offsetMask)
}

func (s *TokenStream) typeAt(index int) int {
	if index < 0 || index >= len(s.offsetAndType) {
		return 0
	}
	return int(int32(s.offsetAndType[index]) >> typeShift)
}

// LookupType returns the type of the token offset tokens after the current one.
func (s *TokenStream) LookupType(offset int) int {
	offset += s.TokenIndex

	if offset < s.TokenCount {
		return s.typeAt(offset)
	}

	return EOF
}

// LookupTypeNonSC returns the type of the idx-th token at or after the current one that is
// neither whitespace nor a comment.
func (s *TokenStream) LookupTypeNonSC(idx int) int {
	for offset := s.TokenIndex; offset < s.TokenCount; offset++ {
		tokenType := s.typeAt(offset)

		if tokenType != WhiteSpace && tokenType != Comment {
			if idx == 0 {
				return tokenType
			}
			idx--
		}
	}

	return EOF
}

// LookupOffset returns the start offset of the token offset tokens after the current one.
func (s *TokenStream) LookupOffset(offset int) int {
	offset += s.TokenIndex

	if offset < s.TokenCount {
		return s.offsetAt(offset - 1)
	}

	return len(s.Source)
}

// LookupOffsetNonSC returns how many tokens ahead the idx-th token that is neither whitespace nor
// a comment is, or EOF when there is none.
func (s *TokenStream) LookupOffsetNonSC(idx int) int {
	for offset := s.TokenIndex; offset < s.TokenCount; offset++ {
		tokenType := s.typeAt(offset)

		if tokenType != WhiteSpace && tokenType != Comment {
			if idx == 0 {
				return offset - s.TokenIndex
			}
			idx--
		}
	}

	return EOF
}

// LookupValue reports whether the text of the token offset tokens after the current one matches
// referenceStr, compared case-insensitively for A..Z.
func (s *TokenStream) LookupValue(offset int, referenceStr string) bool {
	offset += s.TokenIndex

	if offset < s.TokenCount {
		return CmpStr(
			s.Source,
			s.offsetAt(offset-1),
			s.offsetAt(offset),
			referenceStr,
		)
	}

	return false
}

// GetTokenStart returns the start offset of a token by index.
func (s *TokenStream) GetTokenStart(tokenIndex int) int {
	if tokenIndex == s.TokenIndex {
		return s.TokenStart
	}

	if tokenIndex > 0 {
		if tokenIndex < s.TokenCount {
			return s.offsetAt(tokenIndex - 1)
		}
		return s.offsetAt(s.TokenCount)
	}

	return s.FirstCharOffset
}

// GetTokenEnd returns the end offset of a token by index.
func (s *TokenStream) GetTokenEnd(tokenIndex int) int {
	if tokenIndex == s.TokenIndex {
		return s.TokenEnd
	}

	return s.offsetAt(boundIndex(tokenIndex, 0, s.TokenCount))
}

// GetTokenType returns the type of a token by index.
func (s *TokenStream) GetTokenType(tokenIndex int) int {
	if tokenIndex == s.TokenIndex {
		return s.TokenType
	}

	return s.typeAt(boundIndex(tokenIndex, 0, s.TokenCount))
}

// SubstrToCursor returns the source between start and the start of the current token.
func (s *TokenStream) SubstrToCursor(start int) jsutil.Str {
	return s.Source.Substring(start, s.TokenStart)
}

// IsBlockOpenerTokenType reports whether a token type opens a block: a function, <(-token>,
// <[-token> or <{-token>.
func (s *TokenStream) IsBlockOpenerTokenType(tokenType int) bool {
	return blockTokenOf(tokenType) == blockOpenToken
}

// IsBlockCloserTokenType reports whether a token type closes a block: a <)-token>, <]-token> or
// <}-token>.
func (s *TokenStream) IsBlockCloserTokenType(tokenType int) bool {
	return blockTokenOf(tokenType) == blockCloseToken
}

// GetBlockTokenPairIndex returns the index of the token that pairs with the block token at
// tokenIndex, or -1 when there is none.
func (s *TokenStream) GetBlockTokenPairIndex(tokenIndex int) int {
	tokenType := s.GetTokenType(tokenIndex)

	if blockTokenOf(tokenType) == 1 {
		// block open token
		pairIndex := int(s.Balance[tokenIndex])
		closeType := s.GetTokenType(pairIndex)

		if balancePairOf(tokenType) == closeType {
			return pairIndex
		}
		return -1
	} else if blockTokenOf(tokenType) == 2 {
		// block close token
		pairIndex := int(s.Balance[tokenIndex])
		openType := s.GetTokenType(pairIndex)

		if balancePairOf(openType) == tokenType {
			return pairIndex
		}
		return -1
	}

	return -1
}

// IsBalanceEdge reports whether tokenIndex is past the block the current token is in.
func (s *TokenStream) IsBalanceEdge(tokenIndex int) bool {
	return int(s.Balance[s.TokenIndex]) < tokenIndex
}

// IsDelim reports whether the token offset tokens after the current one, or the current one when
// offset is 0, is a <delim-token> holding code.
func (s *TokenStream) IsDelim(code uint16, offset int) bool {
	if offset != 0 {
		return s.LookupType(offset) == Delim &&
			s.Source.CharCodeAt(s.LookupOffset(offset)) == code
	}

	return s.TokenType == Delim &&
		s.Source.CharCodeAt(s.TokenStart) == code
}

// Skip moves tokenCount tokens forward.
func (s *TokenStream) Skip(tokenCount int) {
	next := s.TokenIndex + tokenCount

	if next < s.TokenCount {
		s.TokenIndex = next
		s.TokenStart = s.offsetAt(next - 1)
		s.TokenType = s.typeAt(next)
		s.TokenEnd = s.offsetAt(next)
	} else {
		s.TokenIndex = s.TokenCount
		s.Next()
	}
}

// Next moves to the next token, or to the end of the stream.
func (s *TokenStream) Next() {
	next := s.TokenIndex + 1

	if next < s.TokenCount {
		s.TokenIndex = next
		s.TokenStart = s.TokenEnd
		s.TokenType = s.typeAt(next)
		s.TokenEnd = s.offsetAt(next)
	} else {
		s.EOF = true
		s.TokenIndex = s.TokenCount
		s.TokenType = EOF
		s.TokenEnd = len(s.Source)
		s.TokenStart = s.TokenEnd
	}
}

// SkipSC moves past whitespace and comment tokens.
func (s *TokenStream) SkipSC() {
	for s.TokenType == WhiteSpace || s.TokenType == Comment {
		s.Next()
	}
}

// SkipUntilBalanced moves to the end of the balanced run that starts at startToken, stopping
// earlier when stopConsume says so for the first code point of a token: 0 keeps going, 1 stops
// before that token and 2 stops after it. It returns the index it stopped at.
func (s *TokenStream) SkipUntilBalanced(startToken int, stopConsume func(code uint16) int) int {
	cursor := startToken
	balanceEnd := 0
	offset := 0

loop:
	for ; cursor < s.TokenCount; cursor++ {
		balanceEnd = int(s.Balance[cursor])

		// stop scanning on balance edge that points to offset before start token
		if balanceEnd < startToken {
			break loop
		}

		if cursor > 0 {
			offset = s.offsetAt(cursor - 1)
		} else {
			offset = s.FirstCharOffset
		}

		// check stop condition
		switch stopConsume(s.Source.CharCodeAt(offset)) {
		case 1: // just stop
			break loop

		case 2: // stop & included
			cursor++
			break loop

		default:
			// fast forward to the end of balanced block for an open block tokens
			if s.IsBlockOpenerTokenType(s.typeAt(cursor)) {
				cursor = balanceEnd
			}
		}
	}

	s.Skip(cursor - s.TokenIndex)

	return cursor
}

// ForEachToken calls fn for every token with its type, start offset, end offset and index.
func (s *TokenStream) ForEachToken(fn func(tokenType, start, end, index int)) {
	for i, offset := 0, s.FirstCharOffset; i < s.TokenCount; i++ {
		start := offset
		item := s.offsetAndType[i]
		end := int(item & offsetMask)
		tokenType := int(int32(item) >> typeShift)

		offset = end

		fn(tokenType, start, end, i)
	}
}
