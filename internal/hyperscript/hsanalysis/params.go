package hsanalysis

import (
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/parse"
)

type Parameters struct {
	ProgramOrExpression hscode.JSONMap
	LocationKind        hyperscriptCodeLocationKind
	Component           *Component //may be nil
	Chunk               *parse.ParsedChunkSource
	CodeStartIndex      int32
}

type hyperscriptCodeLocationKind int

const (
	InoxJsAttribute hyperscriptCodeLocationKind = iota
	ComponentUnderscoreAttribute
	UnderscoreAttribute
	ClientSideAttributeInterpolation
	ClientSideTextInterpolation
	HyperscriptScriptElement
	HyperscriptScriptFile
)
