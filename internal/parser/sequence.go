package parser

import (
	"github.com/csstree/csstree/internal/ast"
	"github.com/csstree/csstree/internal/tokenizer"
)

// recognizer is a scope: the reader that turns the next tokens into a node, what to do where
// white space was skipped, and the readers that take the place of the sequence for a function of
// a particular name.
type recognizer struct {
	getNode      func(p *parser, ctx *seqContext) *ast.Node
	onWhiteSpace func(p *parser, next *ast.Node, children *ast.List, ctx *seqContext)
	fns          map[string]func(p *parser, rec *recognizer) *ast.List
}

// seqContext is the context a sequence hands its recognizer, which carries the recognizer itself
// so that a nested sequence is read in the same scope.
type seqContext struct {
	recognizer *recognizer
}

// sequenceFn reads a sequence of nodes in a scope. It is handed to the node readers that hold a
// sequence between brackets so that they read their content the same way their parent did.
type sequenceFn func(rec *recognizer) *ast.List

// readSequence reads nodes in a scope until the recognizer no longer knows what comes next.
func (p *parser) readSequence(rec *recognizer) *ast.List {
	children := ast.NewList()
	space := false
	ctx := &seqContext{recognizer: rec}

	for !p.EOF {
		switch p.TokenType {
		case tokenizer.Comment:
			p.Next()
			continue

		case tokenizer.WhiteSpace:
			space = true
			p.Next()
			continue
		}

		if rec.getNode == nil {
			panic(typeError("recognizer.getNode is not a function"))
		}

		child := rec.getNode(p, ctx)

		if child == nil {
			break
		}

		if space {
			if rec.onWhiteSpace != nil {
				rec.onWhiteSpace(p, child, children, ctx)
			}
			space = false
		}

		children.AppendData(child)
	}

	if space && rec.onWhiteSpace != nil {
		rec.onWhiteSpace(p, nil, children, ctx)
	}

	return children
}
