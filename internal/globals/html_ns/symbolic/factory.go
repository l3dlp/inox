package html_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
)

func init() {
	symbolic.RegisterXMLInterpolationCheckingFunction(
		CreateHTMLNodeFromXMLElement,
		func(n parse.Node, value symbolic.SymbolicValue) (errorMsg string) {
			switch value.(type) {
			case *symbolic.XMLElement, symbolic.StringLike, *symbolic.Int:
				return ""
			default:
				return "only XML elements, string-like and integer values are allowed"
			}
		},
	)
}

func CreateHTMLNodeFromXMLElement(ctx *symbolic.Context, elem *symbolic.XMLElement) *HTMLNode {

	var checkElem func(e *symbolic.XMLElement)
	checkElem = func(e *symbolic.XMLElement) {
		for name, val := range e.Attributes() {
			switch val.(type) {
			case symbolic.StringLike, *symbolic.Int:
			default:
				ctx.AddFormattedSymbolicGoFunctionError("value of attribute '%s' is not accepted for now (%s), use a string or an integer", name, symbolic.Stringify(val))
			}
		}

		for _, child := range e.Children() {
			switch c := child.(type) {
			case *symbolic.XMLElement:
				checkElem(c)
			case symbolic.StringLike, *symbolic.Int:
			default:
				ctx.AddFormattedSymbolicGoFunctionError("value of interpolation is not accepted for now (%s), use a string or an integer", symbolic.Stringify(c))
			}
		}
	}

	checkElem(elem)

	return NewHTMLNode()
}
