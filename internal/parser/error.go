package parser

import (
	"strconv"

	"github.com/csstree/csstree/internal/jsutil"
)

const (
	maxLineLength    = 100
	offsetCorrection = 60
	tabReplacement   = "    "
)

// newSyntaxError builds the error a failed parse raises. Its Message is the message on its own,
// as the original's is; FormattedMessage adds an excerpt of the source with two lines of context
// on either side and a marker under the place that failed, which is what the original's
// formattedMessage reads.
func newSyntaxError(message, source jsutil.Str, offset, line, column, baseLine, baseColumn int) *jsutil.Error {
	return &jsutil.Error{
		Name:       "SyntaxError",
		RawMessage: message,
		Message:    message,
		FormattedMessage: jsutil.Concat(
			jsutil.FromString("Parse error: "),
			message,
			jsutil.FromString("\n"),
			sourceFragment(source, line, column, baseLine, baseColumn, 2),
		),
		SourceFragment: func(extraLines int) jsutil.Str {
			return sourceFragment(source, line, column, baseLine, baseColumn, extraLines)
		},
		Source: source,
		Offset: offset,
		Line:   line,
		Column: column,
	}
}

// sourceFragment writes the lines of the source around the place that failed, each behind its
// line number, with a marker line under the column. Long lines are cut down and elided, tabs are
// widened to four spaces, and numbered lines that would be empty at either end are left out.
func sourceFragment(source jsutil.Str, line, column, baseLine, baseColumn, extraLines int) jsutil.Str {
	prefixed := jsutil.Concat(
		repeatCode('\n', maxInt(baseLine-1, 0)),
		repeatCode(' ', maxInt(baseColumn-1, 0)),
		source,
	)
	lines := splitLines(prefixed)

	startLine := maxInt(1, line-extraLines) - 1
	endLine := minInt(line+extraLines, len(lines)+1)
	maxNumLength := maxInt(4, len(strconv.Itoa(endLine))) + 1
	cutLeft := 0

	// column correction according to replaced tab before column
	column += (len(tabReplacement) - 1) * countTabs(lineAt(lines, line-1), column-1)

	if column > maxLineLength {
		cutLeft = column - offsetCorrection + 3
		column = offsetCorrection - 2
	}

	for i := startLine; i <= endLine; i++ {
		if i >= 0 && i < len(lines) {
			lines[i] = replaceTabs(lines[i])

			var head, tail jsutil.Str
			if cutLeft > 0 && len(lines[i]) > cutLeft {
				head = jsutil.Str{0x2026}
			}
			if len(lines[i]) > cutLeft+maxLineLength-1 {
				tail = jsutil.Str{0x2026}
			}
			lines[i] = jsutil.Concat(head, substr(lines[i], cutLeft, maxLineLength-2), tail)
		}
	}

	parts := []jsutil.Str{
		processLines(lines, startLine, line, maxNumLength),
		jsutil.Concat(repeatCode('-', column+maxNumLength+1), jsutil.Str{'^'}),
		processLines(lines, line, endLine, maxNumLength),
	}

	fragment := jsutil.Str{}
	first := true
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		if !first {
			fragment = jsutil.Concat(fragment, jsutil.Str{'\n'})
		}
		fragment = jsutil.Concat(fragment, part)
		first = false
	}

	fragment = stripLeadingEmptyLines(fragment)
	fragment = stripTrailingEmptyLines(fragment)

	return fragment
}

// processLines writes the lines of the half open range, each behind its padded number.
func processLines(lines []jsutil.Str, start, end, maxNumLength int) jsutil.Str {
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	if end < start {
		end = start
	}

	out := jsutil.Str{}
	for i := start; i < end; i++ {
		if i > start {
			out = jsutil.Concat(out, jsutil.Str{'\n'})
		}
		out = jsutil.Concat(out, padStart(strconv.Itoa(i+1), maxNumLength), jsutil.FromString(" |"), lines[i])
	}
	return out
}

// splitLines cuts a string at every line break, which is a carriage return with an optional line
// feed after it, a line feed on its own, or a form feed.
func splitLines(source jsutil.Str) []jsutil.Str {
	lines := []jsutil.Str{}
	start := 0
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\r':
			lines = append(lines, source.Substring(start, i))
			if i+1 < len(source) && source[i+1] == '\n' {
				i++
			}
			start = i + 1
		case '\n', '\f':
			lines = append(lines, source.Substring(start, i))
			start = i + 1
		}
	}
	lines = append(lines, source.Substring(start, len(source)))
	return lines
}

func lineAt(lines []jsutil.Str, index int) jsutil.Str {
	if index < 0 || index >= len(lines) {
		return nil
	}
	return lines[index]
}

func countTabs(line jsutil.Str, length int) int {
	count := 0
	for i := 0; i < length && i < len(line); i++ {
		if line[i] == '\t' {
			count++
		}
	}
	return count
}

func replaceTabs(line jsutil.Str) jsutil.Str {
	out := make(jsutil.Str, 0, len(line))
	for _, code := range line {
		if code == '\t' {
			out = append(out, ' ', ' ', ' ', ' ')
			continue
		}
		out = append(out, code)
	}
	return out
}

// substr is str.substr(start, length).
func substr(line jsutil.Str, start, length int) jsutil.Str {
	if start < 0 {
		start = 0
	}
	if start > len(line) || length <= 0 {
		return jsutil.Str{}
	}
	end := start + length
	if end > len(line) {
		end = len(line)
	}
	return line.Substring(start, end)
}

func repeatCode(code uint16, count int) jsutil.Str {
	if count <= 0 {
		return jsutil.Str{}
	}
	out := make(jsutil.Str, count)
	for i := range out {
		out[i] = code
	}
	return out
}

func padStart(text string, length int) jsutil.Str {
	out := jsutil.Str{}
	for i := len(text); i < length; i++ {
		out = append(out, ' ')
	}
	return jsutil.Concat(out, jsutil.FromString(text))
}

// isJSSpace reports whether a code unit is one of the characters a JavaScript regular expression
// counts as white space.
func isJSSpace(code uint16) bool {
	switch code {
	case 0x0009, 0x000A, 0x000B, 0x000C, 0x000D, 0x0020, 0x00A0, 0x1680,
		0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return code >= 0x2000 && code <= 0x200A
}

func isASCIIDigit(code uint16) bool {
	return code >= '0' && code <= '9'
}

// numberedLineAt reads one "<spaces><number><space>|" run starting at offset and answers where it
// ends, or -1 when there is none. The three parts take different characters, so reading each of
// them as far as it goes finds the only match there is.
func numberedLineAt(text jsutil.Str, offset int) int {
	start := offset
	for offset < len(text) && isJSSpace(text[offset]) {
		offset++
	}
	if offset == start {
		return -1
	}

	digits := offset
	for offset < len(text) && isASCIIDigit(text[offset]) {
		offset++
	}
	if offset == digits {
		return -1
	}

	spaces := offset
	for offset < len(text) && isJSSpace(text[offset]) {
		offset++
	}
	if offset == spaces {
		return -1
	}

	if offset >= len(text) || text[offset] != '|' {
		return -1
	}
	return offset + 1
}

// stripLeadingEmptyLines drops the numbered lines with nothing on them at the start.
func stripLeadingEmptyLines(text jsutil.Str) jsutil.Str {
	offset := 0
	for {
		end := numberedLineAt(text, offset)
		if end < 0 || end >= len(text) || text[end] != '\n' {
			break
		}
		offset = end + 1
	}
	if offset == 0 {
		return text
	}
	return text.Slice(offset)
}

// stripTrailingEmptyLines drops the numbered lines with nothing on them at the end.
func stripTrailingEmptyLines(text jsutil.Str) jsutil.Str {
	for i := 0; i < len(text); i++ {
		if text[i] != '\n' {
			continue
		}
		offset := i + 1
		matched := false
		for offset < len(text) {
			end := numberedLineAt(text, offset)
			if end < 0 {
				matched = false
				break
			}
			offset = end
			matched = true
		}
		if matched && offset == len(text) {
			return text.Substring(0, i)
		}
	}
	return text
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
