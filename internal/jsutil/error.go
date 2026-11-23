package jsutil

// Error is an error the library raises for a request it understands but cannot serve: a syntax
// error in CSS text or in a value definition, an unknown property or at-rule, or a value that
// does not match its syntax. It carries the name the original gives the error, the message on its
// own, and the full message, which for several of them adds an excerpt of the source with the
// place marked.
type Error struct {
	Name       string
	RawMessage Str
	Message    Str

	// Offset, Line and Column locate a parse error in the source; Offset is -1 when there is none.
	Offset int
	Line   int
	Column int

	// HasMismatch says whether MismatchOffset and MismatchLength locate the part of a value that
	// failed to match its syntax.
	HasMismatch    bool
	MismatchOffset int
	MismatchLength int

	// Source is the text the error was found in, when there is one.
	Source Str

	// FormattedMessage adds an excerpt of the source with the failing place marked, for the
	// errors that carry one; it is empty for the rest.
	FormattedMessage Str

	// SourceFragment writes that excerpt with a chosen number of context lines; it is nil for an
	// error that has no source.
	SourceFragment func(extraLines int) Str
}

// NewError returns an error whose full message is its own message.
func NewError(name string, message Str) *Error {
	return &Error{Name: name, RawMessage: message, Message: message, Offset: -1}
}

// Error is the message, so that an Error can travel as an ordinary Go error.
func (e *Error) Error() string {
	return e.Message.String()
}
