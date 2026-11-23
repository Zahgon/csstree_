// Command csstree is the command line of the css-tree toolkit: it tokenizes, parses, generates,
// walks and searches CSS, reads and writes the value definition syntax, and runs the utilities,
// writing its results to standard output and its diagnostics to standard error.
//
// Run it without arguments for the list of sub-commands.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/cssutil"
	"github.com/csstree/csstree/internal/generator"
	"github.com/csstree/csstree/internal/jsutil"
	"github.com/csstree/csstree/internal/parser"
	"github.com/csstree/csstree/internal/syntaxdef"
	"github.com/csstree/csstree/internal/tokenizer"
	"github.com/csstree/csstree/internal/walker"
)

// usageError is a request the command cannot read: an unknown sub-command or flag, a missing
// argument, an unreadable file or text that is not the JSON it must be.
type usageError struct{ message string }

func (e *usageError) Error() string { return e.message }

func usagef(format string, args ...any) *usageError {
	return &usageError{message: fmt.Sprintf(format, args...)}
}

// out and errOut collect what is written, so that a run that ends in an error writes nothing it
// would have to take back.
type output struct {
	out []byte
	err []byte
}

func (o *output) write(s jsutil.Str)       { o.out = append(o.out, s.UTF8()...) }
func (o *output) writeString(s string)     { o.out = append(o.out, s...) }
func (o *output) writeErr(s string)        { o.err = append(o.err, s...) }
func (o *output) writeErrStr(s jsutil.Str) { o.err = append(o.err, s.UTF8()...) }

// flush writes what was collected. A failure to write is reported on the way out, since a command
// whose output did not reach its destination has not done its work.
func (o *output) flush() error {
	if len(o.out) > 0 {
		if _, err := os.Stdout.Write(o.out); err != nil {
			return fmt.Errorf("writing to standard output: %w", err)
		}
		o.out = nil
	}
	if len(o.err) > 0 {
		if _, err := os.Stderr.Write(o.err); err != nil {
			return fmt.Errorf("writing to standard error: %w", err)
		}
		o.err = nil
	}
	return nil
}

// writeDiagnostic puts one line on standard error, reporting a failure to write as an exit status
// of its own rather than letting it pass unseen.
func writeDiagnostic(line string) int {
	if _, err := os.Stderr.WriteString(line); err != nil {
		return 74 // the output could not be written
	}
	return 0
}

// args reads the flags of a sub-command in any order, then its positional arguments.
type args struct {
	argv []string
	i    int
	rest []string
}

func newArgs(argv []string) *args { return &args{argv: argv} }

func (a *args) next() (string, bool) {
	if a.i < len(a.argv) {
		a.i++
		return a.argv[a.i-1], true
	}
	return "", false
}

func (a *args) value(flag string) (string, error) {
	if a.i >= len(a.argv) {
		return "", usagef("flag %s needs a value", flag)
	}
	a.i++
	return a.argv[a.i-1], nil
}

func (a *args) positional(name string) (string, error) {
	if len(a.rest) == 0 {
		return "", usagef("%s is required", name)
	}
	value := a.rest[0]
	a.rest = a.rest[1:]
	return value, nil
}

func (a *args) optional() (string, bool) {
	if len(a.rest) == 0 {
		return "", false
	}
	value := a.rest[0]
	a.rest = a.rest[1:]
	return value, true
}

func (a *args) end() error {
	if len(a.rest) > 0 {
		return usagef("unexpected argument %s", a.rest[0])
	}
	return nil
}

// parseFlags hands every flag to handle and collects everything else as a positional argument.
// A "--" ends the flags.
func (a *args) parseFlags(handle func(flag string) (bool, error)) error {
	for {
		arg, ok := a.next()
		if !ok {
			return nil
		}
		if arg == "--" {
			for {
				rest, ok := a.next()
				if !ok {
					return nil
				}
				a.rest = append(a.rest, rest)
			}
		}
		if len(arg) > 1 && arg[0] == '-' {
			taken, err := handle(arg)
			if err != nil {
				return err
			}
			if !taken {
				return usagef("unknown flag %s", arg)
			}
			continue
		}
		a.rest = append(a.rest, arg)
	}
}

func readText(name string, present bool) (jsutil.Str, error) {
	if !present || name == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, usagef("cannot read standard input")
		}
		return jsutil.FromUTF8(data), nil
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, usagef("cannot read %s", name)
	}
	return jsutil.FromUTF8(data), nil
}

func readJSON(name string, present bool) (jsutil.Value, error) {
	text, err := readText(name, present)
	if err != nil {
		return nil, err
	}
	value, parseErr := jsutil.ParseJSON(text)
	if parseErr != nil {
		what := "standard input"
		if present && name != "-" {
			what = name
		}
		return nil, usagef("%s is not valid JSON", what)
	}
	return value, nil
}

func intValue(raw, flag string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, usagef("flag %s needs an integer, got %q", flag, raw)
	}
	return n, nil
}

func jsonOf(value jsutil.Value, indent int) jsutil.Str {
	return jsutil.Stringify(value, indent)
}

func jsonString(s jsutil.Str) jsutil.Str {
	return jsutil.Stringify(jsutil.JSONString(s), 0)
}

// parseState holds the parse options a sub-command reading CSS text takes, and what the parse
// reported while it ran.
type parseState struct {
	options  parser.Options
	tolerant bool
	comments bool
	tokens   bool
	lines    []string
	errors   []*jsutil.Error
}

func newParseState() *parseState {
	return &parseState{options: parser.NewOptions()}
}

func (s *parseState) flag(a *args, flag string) (bool, error) {
	switch flag {
	case "--context":
		value, err := a.value(flag)
		if err != nil {
			return true, err
		}
		s.options.Context = value
	case "--positions":
		s.options.Positions = true
	case "--filename":
		value, err := a.value(flag)
		if err != nil {
			return true, err
		}
		s.options.Filename = jsutil.FromString(value)
	case "--offset", "--line", "--column":
		raw, err := a.value(flag)
		if err != nil {
			return true, err
		}
		n, err := intValue(raw, flag)
		if err != nil {
			return true, err
		}
		switch flag {
		case "--offset":
			s.options.Offset = n
		case "--line":
			s.options.Line = n
		case "--column":
			s.options.Column = n
		}
	case "--no-value":
		s.options.ParseValue = false
	case "--no-atrule-prelude":
		s.options.ParseAtrulePrelude = false
	case "--no-rule-prelude":
		s.options.ParseRulePrelude = false
	case "--custom-property":
		s.options.ParseCustomProperty = true
	case "--tolerant":
		s.tolerant = true
	case "--comments":
		s.comments = true
	case "--tokens":
		s.tokens = true
	default:
		return false, nil
	}
	return true, nil
}

// run parses the CSS the sub-command was given and reports what the parse saw.
func (s *parseState) run(o *output, a *args, parse func(jsutil.Str, parser.Options) (*ast.Node, *jsutil.Error)) (*ast.Node, bool, error) {
	name, present := a.optional()
	css, err := readText(name, present)
	if err != nil {
		return nil, false, err
	}
	if err := a.end(); err != nil {
		return nil, false, err
	}

	if s.comments {
		s.options.OnComment = func(value jsutil.Str, loc *ast.Location) {
			start, end := "-", "-"
			if loc != nil {
				start = strconv.Itoa(loc.Start.Offset)
				end = strconv.Itoa(loc.End.Offset)
			}
			s.lines = append(s.lines, fmt.Sprintf("COMMENT\t%s\t%s\t%s", start, end, jsonString(value).String()))
		}
	}
	if s.tokens {
		s.options.OnToken = func(tokenType, start, end int) {
			s.lines = append(s.lines, fmt.Sprintf("TOKEN\t%s\t%d\t%d", tokenizer.Names[tokenType], start, end))
		}
	}
	if s.tolerant {
		s.options.OnParseError = func(err *jsutil.Error, fallback *ast.Node) {
			s.errors = append(s.errors, err)
		}
	}

	tree, parseErr := parse(css, s.options)
	if parseErr != nil {
		return nil, false, parseErr
	}

	for _, line := range s.lines {
		o.writeErr(line + "\n")
	}
	for _, e := range s.errors {
		o.writeErr(fmt.Sprintf("PARSE-ERROR\t%s\t%d\t%d\t%d\t%s\n",
			e.Name, e.Offset, e.Line, e.Column, jsonString(e.RawMessage).String()))
	}

	return tree, len(s.errors) > 0, nil
}

func generateNode(node *ast.Node) jsutil.Str {
	text, err := generator.Generate(node, "")
	if err != nil {
		return jsutil.FromString("<cannot generate " + node.Type + ">")
	}
	return text
}

var contextFields = []string{"Root", "StyleSheet", "Atrule", "AtrulePrelude", "Rule", "Selector", "Block", "Declaration", "Function"}

func contextFlags(ctx *walker.Context) string {
	fields := []*ast.Node{ctx.Root, ctx.StyleSheet, ctx.Atrule, ctx.AtrulePrelude, ctx.Rule, ctx.Selector, ctx.Block, ctx.Declaration, ctx.Function}
	var b strings.Builder
	for _, field := range fields {
		if field != nil {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) (status int) {
	o := &output{}

	defer func() {
		if recovered := recover(); recovered != nil {
			writeDiagnostic(fmt.Sprintf("csstree: internal error: %v\n", recovered))
			status = 70
		}
	}()

	if len(argv) == 0 {
		if code := writeDiagnostic("csstree: a command is required\n"); code != 0 {
			return code
		}
		return 2
	}

	command, ok := commands[argv[0]]
	if !ok {
		if code := writeDiagnostic("csstree: unknown command " + argv[0] + "\n"); code != 0 {
			return code
		}
		return 2
	}

	code, err := command(o, newArgs(argv[1:]))
	if err != nil {
		var usage *usageError
		if u, isUsage := err.(*usageError); isUsage {
			usage = u
		}
		if usage != nil {
			if code := writeDiagnostic("csstree: " + usage.message + "\n"); code != 0 {
				return code
			}
			return 2
		}
		if reported, isReported := err.(*jsutil.Error); isReported {
			if !isReportedName(reported.Name) {
				if code := writeDiagnostic("csstree: internal error: " + reported.Message.String() + "\n"); code != 0 {
					return code
				}
				return 70
			}
			message := reported.RawMessage
			if message == nil {
				message = reported.Message
			}
			o.writeErr(fmt.Sprintf("ERROR\t%s\t%s\n", reported.Name, jsonString(message).String()))
			for _, line := range strings.Split(reported.Message.String(), "\n") {
				o.writeErr("| " + line + "\n")
			}
			if reported.HasMismatch {
				o.writeErr(fmt.Sprintf("MISMATCH\t%d\t%d\n", reported.MismatchOffset, reported.MismatchLength))
			}
			if flushErr := o.flush(); flushErr != nil {
				return writeFailure(flushErr)
			}
			return 1
		}
		if code := writeDiagnostic("csstree: internal error: " + err.Error() + "\n"); code != 0 {
			return code
		}
		return 70
	}

	if flushErr := o.flush(); flushErr != nil {
		return writeFailure(flushErr)
	}
	return code
}

// writeFailure reports that the output could not be written. It is the one failure that cannot be
// reported through the output itself, so when the report cannot be written either, the exit status
// is all that is left to say it.
func writeFailure(err error) int {
	writeDiagnostic("csstree: " + err.Error() + "\n")
	return 74
}

// isReportedName says whether an error names a request the library understood but could not
// serve, which is reported, rather than a failure of the runtime itself, which is internal.
func isReportedName(name string) bool {
	switch name {
	case "Error", "SyntaxError", "SyntaxMatchError", "SyntaxReferenceError":
		return true
	}
	return false
}

type commandFn func(o *output, a *args) (int, error)

// commands are the sub-commands by name. The lexer sub-commands add themselves from their own
// file, so the map is built here rather than assigned.
var commands = map[string]commandFn{}

func init() {
	for name, fn := range map[string]commandFn{
		"tokenize":       cmdTokenize,
		"parse":          cmdParse,
		"parse-selector": cmdParseSelector,
		"generate":       cmdGenerate,
		"roundtrip":      cmdRoundtrip,
		"convert":        cmdConvert,
		"clone":          cmdClone,
		"walk":           cmdWalk,
		"find":           cmdFind,
		"defsyntax":      cmdDefsyntax,
		"util":           cmdUtil,
	} {
		commands[name] = fn
	}
}

func cmdTokenize(o *output, a *args) (int, error) {
	if err := a.parseFlags(func(string) (bool, error) { return false, nil }); err != nil {
		return 0, err
	}
	name, present := a.optional()
	css, err := readText(name, present)
	if err != nil {
		return 0, err
	}
	if err := a.end(); err != nil {
		return 0, err
	}

	stream := tokenizer.NewTokenStream(css, tokenizer.Tokenize)
	i := 0
	for stream.TokenType != tokenizer.EOF {
		start, end := stream.TokenStart, stream.TokenEnd
		o.writeString(fmt.Sprintf("%d\t%s\t%d\t%d\t%d\t%s\n",
			i, tokenizer.Names[stream.TokenType], start, end,
			stream.Balance[stream.TokenIndex], jsonString(css.Substring(start, end)).String()))
		stream.Next()
		i++
	}
	o.writeString(fmt.Sprintf("# tokens %d\n", i))
	return 0, nil
}

func cmdParse(o *output, a *args) (int, error) {
	state := newParseState()
	if err := a.parseFlags(func(flag string) (bool, error) { return state.flag(a, flag) }); err != nil {
		return 0, err
	}
	tree, failed, err := state.run(o, a, parser.Parse)
	if err != nil {
		return 0, err
	}
	o.write(jsonOf(ast.ToPlain(tree), 2))
	o.writeString("\n")
	if failed {
		return 1, nil
	}
	return 0, nil
}

func cmdParseSelector(o *output, a *args) (int, error) {
	state := newParseState()
	if err := a.parseFlags(func(flag string) (bool, error) { return state.flag(a, flag) }); err != nil {
		return 0, err
	}
	tree, failed, err := state.run(o, a, func(css jsutil.Str, opts parser.Options) (*ast.Node, *jsutil.Error) {
		if opts.Context == "" {
			opts.Context = "selectorList"
		}
		return parser.ParseSelector(css, opts)
	})
	if err != nil {
		return 0, err
	}
	o.write(jsonOf(ast.ToPlain(tree), 2))
	o.writeString("\n")
	if failed {
		return 1, nil
	}
	return 0, nil
}

func cmdGenerate(o *output, a *args) (int, error) {
	mode := ""
	if err := a.parseFlags(func(flag string) (bool, error) {
		if flag == "--mode" {
			value, err := a.value(flag)
			if err != nil {
				return true, err
			}
			mode = value
			return true, nil
		}
		return false, nil
	}); err != nil {
		return 0, err
	}
	name, present := a.optional()
	value, err := readJSON(name, present)
	if err != nil {
		return 0, err
	}
	if err := a.end(); err != nil {
		return 0, err
	}
	text, genErr := generator.Generate(ast.FromPlain(value), mode)
	if genErr != nil {
		return 0, genErr
	}
	o.write(text)
	return 0, nil
}

func cmdRoundtrip(o *output, a *args) (int, error) {
	state := newParseState()
	mode := ""
	if err := a.parseFlags(func(flag string) (bool, error) {
		if flag == "--mode" {
			value, err := a.value(flag)
			if err != nil {
				return true, err
			}
			mode = value
			return true, nil
		}
		return state.flag(a, flag)
	}); err != nil {
		return 0, err
	}
	tree, failed, err := state.run(o, a, parser.Parse)
	if err != nil {
		return 0, err
	}
	text, genErr := generator.Generate(tree, mode)
	if genErr != nil {
		return 0, genErr
	}
	o.write(text)
	if failed {
		return 1, nil
	}
	return 0, nil
}

func cmdConvert(o *output, a *args) (int, error) {
	return compareTrees(o, a, func(tree *ast.Node) *ast.Node {
		return ast.FromPlain(ast.ToPlain(tree))
	})
}

func cmdClone(o *output, a *args) (int, error) {
	return compareTrees(o, a, ast.Clone)
}

// compareTrees writes the CSS of a tree put through transform and says whether it is the same as
// the CSS of the tree it came from.
func compareTrees(o *output, a *args, transform func(*ast.Node) *ast.Node) (int, error) {
	state := newParseState()
	if err := a.parseFlags(func(flag string) (bool, error) { return state.flag(a, flag) }); err != nil {
		return 0, err
	}
	tree, failed, err := state.run(o, a, parser.Parse)
	if err != nil {
		return 0, err
	}
	before, genErr := generator.Generate(tree, "")
	if genErr != nil {
		return 0, genErr
	}
	after, genErr := generator.Generate(transform(tree), "")
	if genErr != nil {
		return 0, genErr
	}
	o.write(after)
	o.writeString("\n")
	if before.Equal(after) {
		o.writeString("SAME\n")
	} else {
		o.writeString("DIFF\n")
	}
	if failed {
		return 1, nil
	}
	return 0, nil
}

func cmdWalk(o *output, a *args) (int, error) {
	state := newParseState()
	reverse := false
	visit := ""
	enter := false
	leave := false
	if err := a.parseFlags(func(flag string) (bool, error) {
		switch flag {
		case "--reverse":
			reverse = true
		case "--visit":
			value, err := a.value(flag)
			if err != nil {
				return true, err
			}
			visit = value
		case "--enter":
			enter = true
		case "--leave":
			leave = true
		default:
			return state.flag(a, flag)
		}
		return true, nil
	}); err != nil {
		return 0, err
	}
	tree, failed, err := state.run(o, a, parser.Parse)
	if err != nil {
		return 0, err
	}
	if !enter && !leave {
		enter, leave = true, true
	}

	count := 0
	record := func(kind string) walker.Visitor {
		return func(node *ast.Node, item *ast.Item, list *ast.List, ctx *walker.Context) {
			count++
			inList := 0
			if item != nil {
				inList = 1
			}
			o.writeString(fmt.Sprintf("%s\t%s\t%d\t%s\t%s\n",
				kind, node.Type, inList, contextFlags(ctx), jsonString(generateNode(node)).String()))
		}
	}

	options := walker.Options{Reverse: reverse, Visit: visit}
	if enter {
		options.Enter = record("E")
	}
	if leave {
		options.Leave = record("L")
	}
	if walkErr := walker.Walk(tree, options); walkErr != nil {
		return 0, walkErr
	}
	o.writeString(fmt.Sprintf("# visits %d\n", count))
	if failed {
		return 1, nil
	}
	return 0, nil
}

func cmdFind(o *output, a *args) (int, error) {
	state := newParseState()
	nodeType := ""
	hasType := false
	last := false
	all := false
	if err := a.parseFlags(func(flag string) (bool, error) {
		switch flag {
		case "--type":
			value, err := a.value(flag)
			if err != nil {
				return true, err
			}
			nodeType, hasType = value, true
		case "--last":
			last = true
		case "--all":
			all = true
		default:
			return state.flag(a, flag)
		}
		return true, nil
	}); err != nil {
		return 0, err
	}
	tree, failed, err := state.run(o, a, parser.Parse)
	if err != nil {
		return 0, err
	}

	test := func(node *ast.Node, item *ast.Item, list *ast.List, ctx *walker.Context) bool {
		return !hasType || node.Type == nodeType
	}

	var found []*ast.Node
	switch {
	case all:
		found = walker.FindAll(tree, test)
	case last:
		if node := walker.FindLast(tree, test); node != nil {
			found = []*ast.Node{node}
		}
	default:
		if node := walker.Find(tree, test); node != nil {
			found = []*ast.Node{node}
		}
	}

	if len(found) == 0 {
		o.writeString("# none\n")
	} else {
		for _, node := range found {
			o.write(jsonString(generateNode(node)))
			o.writeString("\n")
		}
	}
	if failed {
		return 1, nil
	}
	return 0, nil
}

func cmdDefsyntax(o *output, a *args) (int, error) {
	sub, ok := a.next()
	if !ok {
		return 0, usagef("unknown defsyntax sub-command (none)")
	}
	compact := false
	forceBraces := false
	flags := func(flag string) (bool, error) {
		switch flag {
		case "--compact":
			compact = true
		case "--force-braces":
			forceBraces = true
		default:
			return false, nil
		}
		return true, nil
	}

	switch sub {
	case "parse":
		if err := a.parseFlags(flags); err != nil {
			return 0, err
		}
		definition, err := a.positional("DEF")
		if err != nil {
			return 0, err
		}
		if err := a.end(); err != nil {
			return 0, err
		}
		node, parseErr := syntaxdef.Parse(jsutil.FromString(definition))
		if parseErr != nil {
			return 0, parseErr
		}
		o.write(jsonOf(syntaxdef.ToPlain(node), 2))
		o.writeString("\n")
		return 0, nil

	case "generate":
		if err := a.parseFlags(flags); err != nil {
			return 0, err
		}
		name, present := a.optional()
		value, err := readJSON(name, present)
		if err != nil {
			return 0, err
		}
		if err := a.end(); err != nil {
			return 0, err
		}
		o.write(syntaxdef.Generate(syntaxdef.FromPlain(value), forceBraces, compact))
		o.writeString("\n")
		return 0, nil

	case "roundtrip":
		if err := a.parseFlags(flags); err != nil {
			return 0, err
		}
		definition, err := a.positional("DEF")
		if err != nil {
			return 0, err
		}
		if err := a.end(); err != nil {
			return 0, err
		}
		node, parseErr := syntaxdef.Parse(jsutil.FromString(definition))
		if parseErr != nil {
			return 0, parseErr
		}
		o.write(syntaxdef.Generate(node, forceBraces, compact))
		o.writeString("\n")
		return 0, nil

	case "walk":
		if err := a.parseFlags(flags); err != nil {
			return 0, err
		}
		definition, err := a.positional("DEF")
		if err != nil {
			return 0, err
		}
		if err := a.end(); err != nil {
			return 0, err
		}
		node, parseErr := syntaxdef.Parse(jsutil.FromString(definition))
		if parseErr != nil {
			return 0, parseErr
		}
		count := 0
		syntaxdef.Walk(node, func(n *syntaxdef.Node) {
			count++
			o.writeString("E\t" + n.Type + "\t" + jsonString(syntaxdef.Generate(n, false, false)).String() + "\n")
		}, func(n *syntaxdef.Node) {
			count++
			o.writeString("L\t" + n.Type + "\t" + jsonString(syntaxdef.Generate(n, false, false)).String() + "\n")
		})
		o.writeString(fmt.Sprintf("# visits %d\n", count))
		return 0, nil
	}

	return 0, usagef("unknown defsyntax sub-command %s", sub)
}

func cmdUtil(o *output, a *args) (int, error) {
	if err := a.parseFlags(func(string) (bool, error) { return false, nil }); err != nil {
		return 0, err
	}
	name, err := a.positional("function")
	if err != nil {
		return 0, err
	}
	arg := func(what string) (jsutil.Str, error) {
		value, err := a.positional(what)
		if err != nil {
			return nil, err
		}
		return jsutil.FromString(value), nil
	}
	offsetArg := func() (int, error) {
		if len(a.rest) == 0 {
			return 0, nil
		}
		raw, _ := a.optional()
		return intValue(raw, "OFFSET")
	}

	switch name {
	case "string-decode", "string-encode", "string-encode-apos",
		"url-decode", "url-encode", "ident-decode", "ident-encode":
		value, err := arg("S")
		if err != nil {
			return 0, err
		}
		var result jsutil.Str
		switch name {
		case "string-decode":
			result = cssutil.DecodeString(value)
		case "string-encode":
			result = cssutil.EncodeString(value, false)
		case "string-encode-apos":
			result = cssutil.EncodeString(value, true)
		case "url-decode":
			result = cssutil.DecodeURL(value)
		case "url-encode":
			result = cssutil.EncodeURL(value)
		case "ident-decode":
			result = cssutil.DecodeIdent(value)
		case "ident-encode":
			result = cssutil.EncodeIdent(value)
		}
		o.write(jsonString(result))
		o.writeString("\n")

	case "keyword":
		value, err := arg("S")
		if err != nil {
			return 0, err
		}
		d := cssutil.Keyword(value)
		object := jsutil.NewObject()
		object.SetString("basename", jsutil.JSONString(d.Basename))
		object.SetString("name", jsutil.JSONString(d.Name))
		object.SetString("prefix", jsutil.JSONString(d.Prefix))
		object.SetString("vendor", jsutil.JSONString(d.Vendor))
		object.SetString("custom", jsutil.Bool(d.Custom))
		o.write(jsonOf(object, 1))
		o.writeString("\n")

	case "property":
		value, err := arg("S")
		if err != nil {
			return 0, err
		}
		d := cssutil.Property(value)
		object := jsutil.NewObject()
		object.SetString("basename", jsutil.JSONString(d.Basename))
		object.SetString("name", jsutil.JSONString(d.Name))
		object.SetString("hack", jsutil.JSONString(d.Hack))
		object.SetString("vendor", jsutil.JSONString(d.Vendor))
		object.SetString("prefix", jsutil.JSONString(d.Prefix))
		object.SetString("custom", jsutil.Bool(d.Custom))
		o.write(jsonOf(object, 1))
		o.writeString("\n")

	case "vendor-prefix":
		value, err := arg("S")
		if err != nil {
			return 0, err
		}
		offset, err := offsetArg()
		if err != nil {
			return 0, err
		}
		o.write(jsonString(cssutil.VendorPrefix(value, offset)))
		o.writeString("\n")

	case "is-custom-property":
		value, err := arg("S")
		if err != nil {
			return 0, err
		}
		offset, err := offsetArg()
		if err != nil {
			return 0, err
		}
		o.writeString(strconv.FormatBool(cssutil.IsCustomProperty(value, offset)) + "\n")

	case "location":
		value, err := arg("S")
		if err != nil {
			return 0, err
		}
		locator := tokenizer.NewOffsetToLocation(value, 0, 1, 1)
		for len(a.rest) > 0 {
			raw, _ := a.optional()
			offset, err := intValue(raw, "OFFSET")
			if err != nil {
				return 0, err
			}
			loc := locator.GetLocation(offset, nil)
			o.writeString(fmt.Sprintf("%d\t%d\t%d\n", offset, loc.Line, loc.Column))
		}

	default:
		return 0, usagef("unknown util function %s", name)
	}

	if err := a.end(); err != nil {
		return 0, err
	}
	return 0, nil
}
