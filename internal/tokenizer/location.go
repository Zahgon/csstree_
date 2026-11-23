package tokenizer

import "github.com/csstree/csstree/internal/jsutil"

const (
	charN = 10
	charF = 12
	charR = 13
)

// Loc is a place in a source: the offset, counted from the start offset the map was given, and
// the line and column of that offset.
type Loc struct {
	Source jsutil.Str
	Offset int
	Line   int
	Column int
}

// OffsetToLocation maps an offset in a source to a line and a column. The line and column of
// every offset are computed once, on the first lookup.
type OffsetToLocation struct {
	source      jsutil.Str
	startOffset int
	startLine   int
	startColumn int
	computed    bool
	lines       []uint32
	columns     []uint32
}

// NewOffsetToLocation creates a map over source, with the offset, line and column the source
// starts at.
func NewOffsetToLocation(source jsutil.Str, startOffset, startLine, startColumn int) *OffsetToLocation {
	o := &OffsetToLocation{}
	o.SetSource(source, startOffset, startLine, startColumn)
	o.lines = nil
	o.columns = nil
	return o
}

// SetSource points the map at another source and drops what it computed for the previous one.
func (o *OffsetToLocation) SetSource(source jsutil.Str, startOffset, startLine, startColumn int) {
	o.source = source
	o.startOffset = startOffset
	o.startLine = startLine
	o.startColumn = startColumn
	o.computed = false
}

func (o *OffsetToLocation) computeLinesAndColumns() {
	source := o.source
	sourceLength := len(source)
	startOffset := 0
	if sourceLength > 0 {
		startOffset = IsBOM(source.CharCodeAt(0))
	}
	lines := adoptBuffer(o.lines, sourceLength)
	columns := adoptBuffer(o.columns, sourceLength)
	line := o.startLine
	column := o.startColumn

	for i := startOffset; i < sourceLength; i++ {
		code := source.CharCodeAt(i)

		lines[i] = uint32(line)
		columns[i] = uint32(column)
		column++

		if code == charN || code == charR || code == charF {
			if code == charR && i+1 < sourceLength && source.CharCodeAt(i+1) == charN {
				i++
				lines[i] = uint32(line)
				columns[i] = uint32(column)
			}

			line++
			column = 1
		}
	}

	lines[sourceLength] = uint32(line)
	columns[sourceLength] = uint32(column)

	o.lines = lines
	o.columns = columns
	o.computed = true
}

// at reads a computed table the way JavaScript reads an array: an offset the computation did not
// reach, which includes the code unit a byte order mark occupies and anything past the end of the
// source, reads as whatever the buffer holds there, and as 0 beyond the buffer itself.
func at(buffer []uint32, offset int) int {
	if offset < 0 || offset >= len(buffer) {
		return 0
	}
	return int(buffer[offset])
}

// GetLocation returns the place of an offset, naming filename as its source.
func (o *OffsetToLocation) GetLocation(offset int, filename jsutil.Str) Loc {
	if !o.computed {
		o.computeLinesAndColumns()
	}

	return Loc{
		Source: filename,
		Offset: o.startOffset + offset,
		Line:   at(o.lines, offset),
		Column: at(o.columns, offset),
	}
}

// GetLocationRange returns the places of the two ends of a range, naming filename as their source.
func (o *OffsetToLocation) GetLocationRange(start, end int, filename jsutil.Str) (Loc, Loc) {
	if !o.computed {
		o.computeLinesAndColumns()
	}

	return Loc{
			Source: filename,
			Offset: o.startOffset + start,
			Line:   at(o.lines, start),
			Column: at(o.columns, start),
		}, Loc{
			Source: filename,
			Offset: o.startOffset + end,
			Line:   at(o.lines, end),
			Column: at(o.columns, end),
		}
}
