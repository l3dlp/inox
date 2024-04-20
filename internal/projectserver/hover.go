package projectserver

import (
	"bufio"
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

type hoverContentParams struct {
	fpath                string
	docURI               defines.DocumentUri
	line, column         int32
	rpcSession           *jsonrpc.Session
	lastCodebaseAnalysis *analysis.Result     //optional
	diagnostics          *documentDiagnostics //may be nil

	memberAuthToken string
	fls             *Filesystem
	chunkCache      *parse.ChunkCache
	project         *project.Project
}

// getHoverContent gets hover content for a specific position in an Inox code file.
func getHoverContent(handlingCtx *core.Context, params hoverContentParams) (*defines.Hover, error) {

	fpath, line, column, rpcSession, memberAuthToken := params.fpath, params.line, params.column, params.rpcSession, params.memberAuthToken

	preparationResult, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
		fpath:                              fpath,
		requiresState:                      true,
		requiresCache:                      true,
		forcePrepareIfNoVeryRecentActivity: true,

		rpcSession:      rpcSession,
		lspFilesystem:   params.fls,
		inoxChunkCache:  params.chunkCache,
		project:         params.project,
		memberAuthToken: memberAuthToken,
	})

	if !ok {
		return &defines.Hover{}, nil
	}

	state := preparationResult.state
	chunk := preparationResult.chunk

	if !preparationResult.cachedOrGotCache && preparationResult.state != nil {
		//teardown in separate goroutine to return quickly
		defer func() {
			go func() {
				defer utils.Recover()
				preparationResult.state.Ctx.CancelGracefully()
			}()
		}()
	}

	if preparationResult.state == nil || state.SymbolicData == nil {
		rpcSession.LoggerPrintln("no data")
		return &defines.Hover{}, nil
	}

	span := chunk.GetLineColumnSingeCharSpan(line, column)
	hoveredNode, ancestors, ok := chunk.GetNodeAndChainAtSpan(span)
	cursorIndex := span.Start

	if !ok || hoveredNode == nil {
		rpcSession.LoggerPrintln("no data")
		return &defines.Hover{}, nil
	}

	//Hyperscript attribute shorthand
	if attribute, ok := hoveredNode.(*parse.HyperscriptAttributeShorthand); ok && attribute.HyperscriptParsingResult != nil {
		help := getHyperscriptHelpMarkdown(attribute, span)
		if help == "" {
			return &defines.Hover{}, nil
		}

		return &defines.Hover{
			Contents: defines.MarkupContent{
				Kind:  defines.MarkupKindMarkdown,
				Value: help,
			},
		}, nil
	}

	//Raw markup element (e.g. <script>, <style>).
	if elem, ok := hoveredNode.(*parse.MarkupElement); ok && elem.RawElementContent != "" {
		help := getRawMarkupElementContentHelpMarkdown(elem, span)
		if help == "" {
			return &defines.Hover{}, nil
		}

		return &defines.Hover{
			Contents: defines.MarkupContent{
				Kind:  defines.MarkupKindMarkdown,
				Value: help,
			},
		}, nil
	}

	//sectionHelp about manifest sections & lthread meta sections
	sectionHelp, ok := getSectionHelp(hoveredNode, ancestors)
	if ok {
		return &defines.Hover{
			Contents: defines.MarkupContent{
				Kind:  defines.MarkupKindMarkdown,
				Value: sectionHelp,
			},
		}, nil
	}

	tagOrAttrHelp, shouldSpecificValBeIgnored, hasTagOrAttrHelp := getTagOrAttributeHoverHelp(hoveredNode, ancestors, cursorIndex, params)

	//Try getting the hovered node's value.
	mostSpecificVal, ok := state.SymbolicData.GetMostSpecificNodeValue(hoveredNode)
	var lessSpecificVal symbolic.Value

	if !ok || shouldSpecificValBeIgnored {
		if hasTagOrAttrHelp {
			return &defines.Hover{
				Contents: defines.MarkupContent{
					Kind:  defines.MarkupKindMarkdown,
					Value: tagOrAttrHelp,
				},
			}, nil
		}

		rpcSession.LoggerPrintln("no data")
		return &defines.Hover{}, nil
	}

	buff := &bytes.Buffer{}
	w := bufio.NewWriterSize(buff, 1000)
	var stringifiedHoveredNodeValue string

	{
		utils.Must(symbolic.PrettyPrint(symbolic.PrettyPrintArgs{
			Value:             mostSpecificVal,
			Writer:            w,
			Config:            HOVER_PRETTY_PRINT_CONFIG,
			Depth:             0,
			ParentIndentCount: 0,
		}))
		var ok bool
		lessSpecificVal, ok = state.SymbolicData.GetLessSpecificNodeValue(hoveredNode)
		if ok {
			w.Write(utils.StringAsBytes("\n\n# less specific\n"))
			utils.Must(symbolic.PrettyPrint(symbolic.PrettyPrintArgs{
				Value:             lessSpecificVal,
				Writer:            w,
				Config:            HOVER_PRETTY_PRINT_CONFIG,
				Depth:             0,
				ParentIndentCount: 0,
			}))
		}

		w.Flush()
		stringifiedHoveredNodeValue = strings.ReplaceAll(buff.String(), "\n\r", "\n")
	}

	//help for most specific & less specific values
	var helpMessage string
	{
		val := mostSpecificVal
		for {
			switch val := val.(type) {
			case *symbolic.GoFunction:
				markdown, ok := help.HelpForSymbolicGoFunc(val, help.HelpMessageConfig{Format: help.MarkdownFormat})
				if ok {
					helpMessage = "\n-----\n" + strings.ReplaceAll(markdown, "\n\r", "\n")
				}
			}
			if helpMessage == "" && val == mostSpecificVal && lessSpecificVal != nil {
				val = lessSpecificVal
				continue
			}
			break
		}
	}

	if hasTagOrAttrHelp {
		helpMessage += "\n\n" + tagOrAttrHelp
	}

	codeBlockWriter := &strings.Builder{}

	if stringifiedHoveredNodeValue != "" {
		codeBlockWriter.WriteString("```inox\n")
		codeBlockWriter.WriteString(stringifiedHoveredNodeValue)
		codeBlockWriter.WriteString("\n```")
	}

	if diagnostics := params.diagnostics; diagnostics != nil {
		codeBlockWriter.WriteString("\n__Reformatted errors:__\n")

		diagnostics.lock.Lock()
		checkErrors := slices.Clone(diagnostics.symbolicErrors[params.docURI])
		diagnostics.lock.Unlock()

		for _, checkError := range checkErrors {
			codeBlockWriter.WriteByte('\n')

			err := checkError.ReformatNonLocated(codeBlockWriter, symbolic.ErrorReformatting{
				BeforeSmallReprInoxValue: "```",
				AfterSmallReprInoxValue:  "```",
				BeforeLongReprInoxValue:  "\n```inox\n",
				AfterLongReprInoxValue:   "\n```\n",
				LongReprThreshold:        20,
				ValuePrettyPringConfig:   HOVER_PRETTY_PRINT_CONFIG,
			})
			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}
		}
	}

	return &defines.Hover{
		Contents: defines.MarkupContent{
			Kind:  defines.MarkupKindMarkdown,
			Value: codeBlockWriter.String() + helpMessage,
		},
	}, nil
}

func getSectionHelp(n parse.Node, ancestors []parse.Node) (string, bool) {
	ancestorCount := len(ancestors)

	if len(ancestors) < 4 {
		return "", false
	}

	//check the hovered node is the key of an object property
	objProp, ok := ancestors[ancestorCount-1].(*parse.ObjectProperty)
	if !ok || objProp.Key != n || !utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-2]) {
		return "", false
	}

	object := ancestors[ancestorCount-2].(*parse.ObjectLiteral)
	propName := objProp.Name()
	grandparent := ancestors[ancestorCount-3]

	switch gp := grandparent.(type) {
	case *parse.Manifest:
		sectionName := propName
		//hovered node is a manifest section's name
		help, ok := help.HelpFor(fmt.Sprintf("manifest/%s-section", sectionName), help.HelpMessageConfig{
			Format: help.MarkdownFormat,
		})

		if ok {
			return help, true
		}
	case *parse.ImportStatement:
		sectionName := propName
		//hovered node is a module import section's name
		help, ok := help.HelpFor(fmt.Sprintf("module-import-config/%s-section", sectionName), help.HelpMessageConfig{
			Format: help.MarkdownFormat,
		})

		if ok {
			return help, true
		}
	case *parse.SpawnExpression:
		sectionName := propName
		if object == gp.Meta {
			//hovered node is a lthread meta section's name
			help, ok := help.HelpFor(fmt.Sprintf("lthreads/%s-section", sectionName), help.HelpMessageConfig{
				Format: help.MarkdownFormat,
			})
			if ok {
				return help, true
			}
		}
	case *parse.ObjectProperty:
		//hovered node is a property name of a database description
		if ancestorCount >= 7 && utils.Implements[*parse.Manifest](ancestors[ancestorCount-7]) &&
			utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-6]) &&
			utils.Implements[*parse.ObjectProperty](ancestors[ancestorCount-5]) &&
			ancestors[ancestorCount-5].(*parse.ObjectProperty).HasNameEqualTo(inoxconsts.MANIFEST_DATABASES_SECTION_NAME) &&
			utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-4]) &&
			utils.Implements[*parse.ObjectProperty](ancestors[ancestorCount-3]) &&
			utils.Implements[*parse.ObjectLiteral](ancestors[ancestorCount-2]) {

			descPropName := propName

			help, ok := help.HelpFor("manifest/databases-section/"+descPropName, help.HelpMessageConfig{
				Format: help.MarkdownFormat,
			})

			if ok {
				return help, true
			}
		}
	}
	return "", false
}
