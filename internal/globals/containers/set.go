package containers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/inoxlang/inox/internal/commonfmt"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	jsoniter "github.com/json-iterator/go"
	"github.com/oklog/ulid/v2"

	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

var (
	ErrSetCanOnlyContainRepresentableValues = errors.New("a Set can only contain representable values")
	ErrValueDoesMatchElementPattern         = errors.New("provided value does not match the element pattern")
	ErrValueWithSameKeyAlreadyPresent       = errors.New("provided value has the same key as an already present element")

	_ core.DefaultValuePattern = (*SetPattern)(nil)
)

func init() {
	core.RegisterLoadInstanceFn(reflect.TypeOf((*SetPattern)(nil)), loadSet)
}

type Set struct {
	elements map[string]core.Serializable
	config   SetConfig
	pattern  *SetPattern

	//persistence
	storage core.SerializedValueStorage //nillable
	url     core.URL                    //set if .storage set
	path    core.Path

	core.NotClonableMixin
}

func NewSet(ctx *core.Context, elements core.Iterable, configObject ...*core.Object) *Set {
	config := SetConfig{
		Uniqueness: containers_common.UniquenessConstraint{
			Type: containers_common.UniqueRepr,
		},
		Element: core.SERIALIZABLE_PATTERN,
	}

	if len(configObject) > 0 {
		obj := configObject[0]
		obj.ForEachEntry(func(k string, v core.Serializable) error {
			switch k {
			case coll_symbolic.SET_CONFIG_ELEMENT_PATTERN_PROP_KEY:
				pattern, ok := v.(core.Pattern)
				if !ok {
					panic(commonfmt.FmtInvalidValueForPropXOfArgY(k, "configuration", "a pattern is expected"))
				}
				config.Element = pattern
			case coll_symbolic.SET_CONFIG_UNIQUE_PROP_KEY:
				switch val := v.(type) {
				case core.Identifier:
					if val == "url" {
						config.Uniqueness.Type = containers_common.UniqueURL
					} else {
						panic(commonfmt.FmtInvalidValueForPropXOfArgY(k, "configuration", "?"))
					}
				case core.PropertyName:
					config.Uniqueness.Type = containers_common.UniquePropertyValue
					config.Uniqueness.PropertyName = val
				default:
					panic(commonfmt.FmtInvalidValueForPropXOfArgY(k, "configuration", "?"))
				}
			default:
				panic(commonfmt.FmtUnexpectedPropInArgX(k, "configuration"))
			}
			return nil
		})
	}

	set := NewSetWithConfig(ctx, elements, config)
	set.pattern = utils.Must(SET_PATTERN.Call([]core.Serializable{set.config.Element, set.config.Uniqueness.ToValue()})).(*SetPattern)
	return set
}

func loadSet(ctx *core.Context, args core.InstanceLoadArgs) (core.UrlHolder, error) {
	path := args.Key
	pattern := args.Pattern
	storage := args.Storage

	setPattern := pattern.(*SetPattern)
	rootData, ok := storage.GetSerialized(ctx, path)
	if !ok {
		if args.AllowMissing {
			rootData = "[]"
		} else {
			return nil, fmt.Errorf("%w: %s", core.ErrFailedToLoadNonExistingValue, path)
		}
	}

	set := NewSetWithConfig(ctx, nil, setPattern.config)
	set.pattern = setPattern
	set.storage = storage
	set.path = path
	set.url = storage.BaseURL().AppendAbsolutePath(path)

	var finalErr error

	//TODO: lazy load
	it := jsoniter.ParseString(jsoniter.ConfigCompatibleWithStandardLibrary, rootData)
	it.ReadArrayCB(func(i *jsoniter.Iterator) (cont bool) {
		val, err := core.ParseNextJSONRepresentation(ctx, it, setPattern.config.Element)
		if err != nil {
			finalErr = fmt.Errorf("failed to parse representation of one of the Set's element: %w", err)
			return false
		}

		defer func() {
			e := recover()

			if err, ok := e.(error); ok {
				finalErr = err
			} else if e != nil {
				cont = false
				finalErr = fmt.Errorf("%#v", e)
			}
		}()
		set.addNoPersist(ctx, val)
		if val.IsMutable() {
			watchable, ok := val.(core.Watchable)
			if !ok {
				finalErr = fmt.Errorf("element should either be immutable or watchable")
				cont = false
				return
			}
			watchable.OnMutation(ctx, func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
				persistSet(ctx, set, set.path, set.storage)
				registerAgain = true
				return
			}, core.MutationWatchingConfiguration{Depth: core.DeepWatching})
		}
		return true
	})

	if finalErr != nil {
		return nil, finalErr
	}

	return set, nil
}

func persistSet(ctx *core.Context, set *Set, path core.Path, storage core.SerializedValueStorage) error {
	stream := jsoniter.NewStream(jsoniter.ConfigCompatibleWithStandardLibrary, nil, 0)
	set.WriteJSONRepresentation(ctx, stream, core.JSONSerializationConfig{
		ReprConfig: &core.ReprConfig{
			AllVisible: true,
		},
		Pattern: set.pattern,
	}, 9)

	storage.SetSerialized(ctx, path, string(stream.Buffer()))
	return nil
}

func (set *Set) WriteRepresentation(ctx *core.Context, w io.Writer, config *core.ReprConfig, depth int) error {
	if depth > core.MAX_REPR_WRITING_DEPTH {
		return core.ErrMaximumReprWritingDepthReached
	}

	buff := bytes.NewBufferString("[")

	first := true
	for _, e := range set.elements {
		if !first {
			buff.WriteByte(',')
		}
		first = false

		if err := e.WriteRepresentation(ctx, buff, config, depth+1); err != nil {
			return err
		}
	}

	buff.WriteByte(']')
	_, err := w.Write(buff.Bytes())
	return err
}

func (set *Set) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	w.WriteArrayStart()

	first := true
	for _, e := range set.elements {
		if !first {
			w.WriteMore()
		}
		first = false

		if err := e.WriteJSONRepresentation(ctx, w, core.JSONSerializationConfig{
			Pattern:    set.config.Element,
			ReprConfig: config.ReprConfig,
		}, depth+1); err != nil {
			return err
		}
	}

	w.WriteArrayEnd()
	return nil
}

type SetConfig struct {
	Element    core.Pattern
	Uniqueness containers_common.UniquenessConstraint
}

func (c SetConfig) Equal(ctx *core.Context, otherConfig SetConfig, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if !c.Uniqueness.Equal(otherConfig.Uniqueness) {
		return false
	}

	//TODO: check Repr config
	if (c.Element == nil) != (otherConfig.Element == nil) {
		return false
	}

	return c.Element == nil || c.Element.Equal(ctx, otherConfig.Element, alreadyCompared, depth+1)
}

func NewSetWithConfig(ctx *core.Context, elements core.Iterable, config SetConfig) *Set {
	set := &Set{
		elements: make(map[string]core.Serializable),
		config:   config,
	}

	if elements != nil {
		it := elements.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			set.Add(ctx, e.(core.Serializable))
		}
	}

	return set
}

func (set *Set) URL() (core.URL, bool) {
	if set.storage != nil {
		return set.url, true
	}
	return "", false
}

func (set *Set) SetURLOnce(ctx *core.Context, url core.URL) error {
	return core.ErrValueDoesNotAcceptURL
}

func (set *Set) Has(ctx *core.Context, elem core.Serializable) core.Bool {
	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	key := containers_common.GetUniqueKey(ctx, elem, set.config.Uniqueness)
	_, ok := set.elements[key]
	return core.Bool(ok)
}

func (set *Set) Get(ctx *core.Context, keyVal core.StringLike) (core.Value, core.Bool) {
	key := keyVal.GetOrBuildString()
	elem, ok := set.elements[key]
	if !ok {
		return nil, false
	}

	return elem, true
}

func (set *Set) Add(ctx *core.Context, elem core.Serializable) {
	set.addNoPersist(ctx, elem)
	//TODO: fully support transaction (in-memory changes)

	if set.storage != nil {
		utils.PanicIfErr(persistSet(ctx, set, set.path, set.storage))
	}
}

func (set *Set) addNoPersist(ctx *core.Context, elem core.Serializable) {
	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	if set.config.Uniqueness.Type == containers_common.UniqueURL {
		holder, ok := elem.(core.UrlHolder)
		if !ok {
			panic(errors.New("elements should be URL holders"))
		}

		_, ok = holder.URL()
		if !ok {
			if set.storage == nil {
				panic(containers_common.ErrFailedGetUniqueKeyNoURL)
			}

			//if the Set is persisted & the elements are unique by URL
			//we set the url of the new element to set.url + '/' + random ID

			url := set.url.ToDirURL().AppendAbsolutePath(core.Path("/" + ulid.Make().String()))
			utils.PanicIfErr(holder.SetURLOnce(ctx, url))
		}
	}

	key := containers_common.GetUniqueKey(ctx, elem, set.config.Uniqueness)

	curr, ok := set.elements[key]
	if ok && elem != curr {
		panic(ErrValueWithSameKeyAlreadyPresent)
	}
	set.elements[key] = elem
}

func (set *Set) Remove(ctx *core.Context, elem core.Serializable) {
	key := containers_common.GetUniqueKey(ctx, elem, set.config.Uniqueness)
	delete(set.elements, key)

	//TODO: fully support transaction (in-memory changes)

	if set.storage != nil {
		utils.PanicIfErr(persistSet(ctx, set, set.path, set.storage))
	}
}

func (f *Set) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "has":
		return core.WrapGoMethod(f.Has), true
	case "add":
		return core.WrapGoMethod(f.Add), true
	case "remove":
		return core.WrapGoMethod(f.Remove), true
	case "get":
		return core.WrapGoMethod(f.Get), true
	}
	return nil, false
}

func (s *Set) Prop(ctx *core.Context, name string) core.Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*Set) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Set) PropertyNames(ctx *core.Context) []string {
	return coll_symbolic.SET_PROPNAMES
}

type SetPattern struct {
	config SetConfig

	core.CallBasedPatternReprMixin

	core.NotCallablePatternMixin
	core.NotClonableMixin
}

func NewSetPattern(config SetConfig, callData core.CallBasedPatternReprMixin) *SetPattern {
	if config.Element == nil {
		config.Element = core.SERIALIZABLE_PATTERN
	}
	return &SetPattern{
		config:                    config,
		CallBasedPatternReprMixin: callData,
	}
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
