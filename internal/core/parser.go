package core

import (
	"encoding/json"
	"errors"

	yamlLex "github.com/goccy/go-yaml/lexer"
	yamlParse "github.com/goccy/go-yaml/parser"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	parsers = map[Mimetype]StatelessParser{}
	_       = []StatelessParser{&jsonParser{}}
)

func init() {
	RegisterParser(mimeconsts.JSON_CTYPE, &jsonParser{})
	RegisterParser(mimeconsts.IXON_CTYPE, &inoxReprParser{})
	RegisterParser(mimeconsts.APP_YAML_CTYPE, &yamlParser{})
}

type StatelessParser interface {
	Validate(ctx *Context, s string) bool
	Parse(ctx *Context, s string) (Serializable, error)
}

func RegisterParser(mime Mimetype, p StatelessParser) {
	if _, ok := parsers[mime]; ok {
		panic(errors.New("a parser is already registered for mime " + string(mime)))
	}
	parsers[mime] = p
}

func GetParser(mime Mimetype) (StatelessParser, bool) {
	p, ok := parsers[mime]
	return p, ok
}

type jsonParser struct {
}

func (p *jsonParser) Validate(ctx *Context, s string) bool {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		panic(ErrTestedStringTooLarge)
	}
	return json.Valid(utils.StringAsBytes(s))

}
func (p *jsonParser) Parse(ctx *Context, s string) (Serializable, error) {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		return nil, ErrTestedStringTooLarge
	}

	var jsonVal any
	err := json.Unmarshal(utils.StringAsBytes(s), &jsonVal)
	if err != nil {
		return nil, err
	}
	return ConvertJSONValToInoxVal(ctx, jsonVal, false), nil
}

type inoxReprParser struct {
}

func (p *inoxReprParser) Validate(ctx *Context, s string) bool {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		panic(ErrTestedStringTooLarge)
	}

	_, err := ParseRepr(ctx, utils.StringAsBytes(s))
	return err == nil

}
func (p *inoxReprParser) Parse(ctx *Context, s string) (Serializable, error) {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		return nil, ErrTestedStringTooLarge
	}

	return ParseRepr(ctx, utils.StringAsBytes(s))
}

type yamlParser struct {
}

func (p *yamlParser) Validate(ctx *Context, s string) bool {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		panic(ErrTestedStringTooLarge)
	}

	tokens := yamlLex.Tokenize(s)
	_, err := yamlParse.Parse(tokens, yamlParse.ParseComments)
	return err == nil
}

func (p *yamlParser) Parse(ctx *Context, s string) (Serializable, error) {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		return nil, ErrTestedStringTooLarge
	}

	tokens := yamlLex.Tokenize(s)
	yml, err := yamlParse.Parse(tokens, 0)

	if err != nil {
		return nil, err
	}
	return ConvertYamlParsedFileToInoxVal(ctx, yml, false), nil
}
