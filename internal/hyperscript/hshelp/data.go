package hshelp

import (
	_ "embed"
	"log"

	"github.com/goccy/go-yaml"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/utils"
)

//go:embed hyperscript.yaml
var HELP_DATA_YAML string

var HELP_DATA struct {
	ByTokenValue map[string]string           `yaml:"token-values"`
	ByTokenType  map[hscode.TokenType]string `yaml:"token-types"`
}

func init() {
	if err := yaml.Unmarshal(utils.StringAsBytes(HELP_DATA_YAML), &HELP_DATA); err != nil {
		log.Panicf("error while parsing hyperscript.yaml: %s", err)
	}
}
