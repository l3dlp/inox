package setcoll

import (
	"path/filepath"
	"reflect"
	"slices"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/jsoniter"
)

const (
	SERIALIZED_SET_PATTERN_ELEM_KEY       = "element"
	SERIALIZED_SET_PATTERN_UNIQUENESS_KEY = "uniqueness"
)

var (
	SET_PATTERN = &core.TypePattern{
		Name:          "Set",
		Type:          reflect.TypeOf((*Set)(nil)),
		SymbolicValue: coll_symbolic.ANY_SET,
		CallImpl: func(typePattern *core.TypePattern, values []core.Serializable) (core.Pattern, error) {
			switch len(values) {
			case 0:
				return nil, commonfmt.FmtMissingArgument("element pattern")
			case 1:
				return nil, commonfmt.FmtMissingArgument("uniqueness")
			}

			elementPattern, ok := values[0].(core.Pattern)
			if !ok {
				return nil, core.FmtErrInvalidArgumentAtPos(elementPattern, 0)
			}

			uniqueness, ok := common.UniquenessConstraintFromValue(values[1])
			if !ok {
				return nil, core.FmtErrInvalidArgumentAtPos(elementPattern, 1)
			}

			return NewSetPattern(SetConfig{
				Element:    elementPattern,
				Uniqueness: uniqueness,
			}, core.CallBasedPatternReprMixin{
				Callee: typePattern,
				Params: slices.Clone(values),
			}), nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.Value) (symbolic.Pattern, error) {
			switch len(values) {
			case 0:
				return nil, commonfmt.FmtMissingArgument("element pattern")
			case 1:
				return nil, commonfmt.FmtMissingArgument("uniqueness")
			}

			elementPattern, ok := values[0].(symbolic.Pattern)
			if !ok {
				return nil, commonfmt.FmtErrInvalidArgumentAtPos(0, "a pattern is expected")
			}

			uniqueness, err := common.UniquenessConstraintFromSymbolicValue(values[1], elementPattern)
			if err != nil {
				return nil, commonfmt.FmtErrInvalidArgumentAtPos(1, err.Error())
			}

			return coll_symbolic.NewSetPatternWithElementPatternAndUniqueness(elementPattern, &uniqueness), nil
		},
	}

	_ core.DefaultValuePattern   = (*SetPattern)(nil)
	_ core.MigrationAwarePattern = (*SetPattern)(nil)
)

type SetPattern struct {
	config SetConfig

	core.NotCallablePatternMixin
}

func NewSetPattern(config SetConfig, callData core.CallBasedPatternReprMixin) *SetPattern {
	if config.Element == nil {
		config.Element = core.SERIALIZABLE_PATTERN
	}
	return &SetPattern{
		config: config,
	}
}

func (p *SetPattern) IsMutable() bool {
	return false
}

func (p *SetPattern) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*SetPattern)
	return ok && p.config.Equal(ctx, otherPatt.config, alreadyCompared, depth+1)
}

func (patt *SetPattern) Test(ctx *core.Context, v core.Value) bool {
	set, ok := v.(*Set)
	if !ok {
		return false
	}

	return patt.config.Equal(ctx, set.config, map[uintptr]uintptr{}, 0)
}
func (p *SetPattern) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (p *SetPattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(core.ErrNotImplementedYet)
}

func (p *SetPattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}

func (p *SetPattern) DefaultValue(ctx *core.Context) (core.Value, error) {
	return NewSetWithConfig(ctx, nil, p.config), nil
}

func (p *SetPattern) GetMigrationOperations(ctx *core.Context, next core.Pattern, pseudoPath string) ([]core.MigrationOp, error) {
	nextSet, ok := next.(*SetPattern)
	if !ok || nextSet.config.Uniqueness != p.config.Uniqueness {
		return []core.MigrationOp{core.ReplacementMigrationOp{
			Current:        p,
			Next:           next,
			MigrationMixin: core.MigrationMixin{PseudoPath: pseudoPath},
		}}, nil
	}

	return core.GetMigrationOperations(ctx, p.config.Element, nextSet.config.Element, filepath.Join(pseudoPath, "*"))
}

func (p *SetPattern) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	if depth > core.MAX_JSON_REPR_WRITING_DEPTH {
		return core.ErrMaximumJSONReprWritingDepthReached
	}

	w.WriteObjectStart()
	w.WriteObjectField(SERIALIZED_SET_PATTERN_ELEM_KEY)

	elemConfig := core.JSONSerializationConfig{ReprConfig: config.ReprConfig}
	err := p.config.Element.WriteJSONRepresentation(ctx, w, elemConfig, depth+1)
	if err != nil {
		return err
	}

	w.WriteMore()
	w.WriteObjectField(SERIALIZED_SET_PATTERN_UNIQUENESS_KEY)
	p.config.Uniqueness.WriteJSONRepresentation(w)

	w.WriteObjectEnd()
	return nil
}
