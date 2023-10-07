package symbolic

import (
	"bufio"
	"reflect"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	FROM_XML_FACTORY_NAME = "from_xml_elem"
)

var (
	ANY_XML_ELEM = &XMLElement{}

	xmlInterpolationCheckingFunctions = map[uintptr] /* go symbolic function pointer*/ XMLInterpolationCheckingFunction{}
)

type XMLInterpolationCheckingFunction func(n parse.Node, value SymbolicValue) (errorMsg string)

func RegisterXMLInterpolationCheckingFunction(factory any, fn XMLInterpolationCheckingFunction) {
	xmlInterpolationCheckingFunctions[reflect.ValueOf(factory).Pointer()] = fn
}

func UnregisterXMLCheckingFunction(factory any) {
	delete(xmlInterpolationCheckingFunctions, reflect.ValueOf(factory).Pointer())
}

// A XMLElement represents a symbolic XMLElement.
type XMLElement struct {
	name       string //if "" matches any node value
	attributes map[string]SymbolicValue
	children   []SymbolicValue
}

func NewXmlElement(name string, attributes map[string]SymbolicValue, children []SymbolicValue) *XMLElement {
	return &XMLElement{name: name, children: children, attributes: attributes}
}

func (e *XMLElement) Name() string {
	return e.name
}

// result should not be modified.
func (e *XMLElement) Attributes() map[string]SymbolicValue {
	return e.attributes
}

// result should not be modified.
func (e *XMLElement) Children() []SymbolicValue {
	return e.children
}

func (r *XMLElement) Test(v SymbolicValue) bool {
	switch val := v.(type) {
	case Writable:
		return true
	default:
		return extData.IsWritable(val)
	}
}

func (r *XMLElement) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%xml-element")))
	return
}

func (r *XMLElement) Writer() *Writer {
	return &Writer{}
}

func (r *XMLElement) WidestOfType() SymbolicValue {
	return ANY_XML_ELEM
}
