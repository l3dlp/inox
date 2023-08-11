package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	openDbFnRegistry     = map[Scheme]OpenDBFn{}
	openDbFnRegistryLock sync.Mutex

	staticallyCheckDbResolutionDataFnRegistry     = map[Scheme]StaticallyCheckDbResolutionDataFn{}
	staticallyCheckDbResolutionDataFnRegistryLock sync.Mutex

	ErrNonUniqueDbOpenFnRegistration                = errors.New("non unique open DB function registration")
	ErrOwnerStateAlreadySet                         = errors.New("owner state already set")
	ErrOwnerStateNotSet                             = errors.New("owner state not set")
	ErrNameCollisionWithInitialDatabasePropertyName = errors.New("name collision with initial database property name")
	ErrTopLevelEntitiesAlreadyLoaded                = errors.New("top-level entities already loaded")
	ErrDatabaseSchemaOnlyUpdatableByOwnerState      = errors.New("database schema can only be updated by owner state")
	ErrNoneDatabaseSchemaUpdateExpected             = errors.New("none database schema update is expected")
	ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed     = errors.New("database schema already updated or no longer allowed")
	ErrInvalidAccessSchemaNotUpdatedYet             = errors.New("access to database is not allowed because schema is not updated yet")

	DATABASE_PROPNAMES = []string{"update_schema", "close", "schema"}

	_ Value    = (*DatabaseIL)(nil)
	_ Database = (*FailedToOpenDatabase)(nil)
)

type DatabaseIL struct {
	inner                Database
	initialSchema        *ObjectPattern
	schemaUpdateExpected bool
	schemaUpdated        atomic.Bool
	schemaUpdateLock     sync.Mutex
	ownerState           *GlobalState //optional, can be set later using .SetOwnerStateOnce

	propertyNames    []string
	topLevelEntities map[string]Serializable
}

type DbOpenConfiguration struct {
	Resource       SchemeHolder
	ResolutionData Value
	FullAccess     bool
}

type OpenDBFn func(ctx *Context, config DbOpenConfiguration) (Database, error)

type StaticallyCheckDbResolutionDataFn func(node parse.Node) (errorMsg string)

type Database interface {
	Resource() SchemeHolder

	Schema() *ObjectPattern

	//UpdateSchema updates the schema and validates the content of the database,
	//this method should return ErrTopLevelEntitiesAlreadyLoaded if it is called after .TopLevelEntities.
	//The caller should always pass a schema whose ALL entry patterns have a loading function.
	UpdateSchema(ctx *Context, schema *ObjectPattern, migrationHandlers MigrationHandlers)

	TopLevelEntities(ctx *Context) map[string]Serializable
	Close(ctx *Context) error
}

type DatabaseWrappingArgs struct {
	Inner                Database
	OwnerState           *GlobalState
	ExpectedSchemaUpdate bool
}

func WrapDatabase(ctx *Context, args DatabaseWrappingArgs) *DatabaseIL {
	schema := args.Inner.Schema()

	propertyNames := utils.CopySlice(DATABASE_PROPNAMES)
	schema.ForEachEntry(func(propName string, propPattern Pattern, isOptional bool) error {
		if utils.SliceContains(DATABASE_PROPNAMES, propName) {
			panic(fmt.Errorf("%w: %s", ErrNameCollisionWithInitialDatabasePropertyName, propName))
		}
		propertyNames = append(propertyNames, propName)
		return nil
	})

	db := &DatabaseIL{
		inner:                args.Inner,
		initialSchema:        schema,
		propertyNames:        propertyNames,
		schemaUpdateExpected: args.ExpectedSchemaUpdate,
		ownerState:           args.OwnerState,
	}

	if !args.ExpectedSchemaUpdate {
		db.topLevelEntities = args.Inner.TopLevelEntities(ctx)
	}

	return db
}

func RegisterOpenDbFn(scheme Scheme, fn OpenDBFn) {
	openDbFnRegistryLock.Lock()
	defer openDbFnRegistryLock.Unlock()

	_, ok := openDbFnRegistry[scheme]
	if ok {
		panic(ErrNonUniqueDbOpenFnRegistration)
	}

	openDbFnRegistry[scheme] = fn
}

func GetOpenDbFn(scheme Scheme) (OpenDBFn, bool) {
	openDbFnRegistryLock.Lock()
	defer openDbFnRegistryLock.Unlock()

	//TODO: prevent re-opening database (same resolution data)
	fn, ok := openDbFnRegistry[scheme]

	return fn, ok
}

func RegisterStaticallyCheckDbResolutionDataFn(scheme Scheme, fn StaticallyCheckDbResolutionDataFn) {
	staticallyCheckDbResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckDbResolutionDataFnRegistryLock.Unlock()

	_, ok := staticallyCheckDbResolutionDataFnRegistry[scheme]
	if ok {
		panic(ErrNonUniqueDbOpenFnRegistration)
	}

	staticallyCheckDbResolutionDataFnRegistry[scheme] = fn
}

func GetStaticallyCheckDbResolutionDataFn(scheme Scheme) (StaticallyCheckDbResolutionDataFn, bool) {
	staticallyCheckDbResolutionDataFnRegistryLock.Lock()
	defer staticallyCheckDbResolutionDataFnRegistryLock.Unlock()

	fn, ok := staticallyCheckDbResolutionDataFnRegistry[scheme]

	return fn, ok
}

func (db *DatabaseIL) SetOwnerStateOnce(state *GlobalState) {
	if db.ownerState != nil {
		panic(ErrOwnerStateAlreadySet)
	}
	db.ownerState = state
}

func (db *DatabaseIL) Resource() SchemeHolder {
	return db.inner.Resource()
}

type MigrationHandlers struct {
	deletions       map[PathPattern]*InoxFunction
	inclusions      map[PathPattern]*InoxFunction
	replacements    map[PathPattern]*InoxFunction
	initializations map[PathPattern]*InoxFunction
}

func (db *DatabaseIL) UpdateSchema(ctx *Context, nextSchema *ObjectPattern, migrations ...*Object) {
	if db.ownerState == nil {
		panic(ErrOwnerStateNotSet)
	}

	if !db.schemaUpdateExpected {
		panic(ErrNoneDatabaseSchemaUpdateExpected)
	}

	if ctx.GetClosestState() != db.ownerState {
		panic(ErrDatabaseSchemaOnlyUpdatableByOwnerState)
	}

	db.schemaUpdateLock.Lock()
	defer db.schemaUpdateLock.Unlock()

	if db.schemaUpdated.Load() {
		panic(ErrDatabaseSchemaAlreadyUpdatedOrNotAllowed)
	}

	defer db.schemaUpdated.Store(true)

	err := nextSchema.ForEachEntry(func(propName string, propPattern Pattern, isOptional bool) error {
		if !hasTypeLoadingFunction(propPattern) {
			return fmt.Errorf("failed to update schema: pattern of .%s has no loading function: %w", propName, ErrNoLoadInstanceFnRegistered)
		}
		return nil
	})

	if err != nil {
		panic(err)
	}

	migrationOps, err := GetMigrationOperations(ctx, db.initialSchema, nextSchema, "/")
	if err != nil {
		panic(err)
	}

	var migrationHandlers MigrationHandlers

	//TODO: make sure concrete migration handlers match concretized symbolic expected migration handlers
	if len(migrationOps) > 0 {
		if len(migrations) != 1 {
			panic(ErrUnreachable)
		}

		migrationObject := migrations[0]
		migrationObject.ForEachEntry(func(k string, v Serializable) error {
			dict := v.(*Dictionary)

			switch k {
			case symbolic.DB_MIGRATION__DELETIONS_PROP_NAME:
				migrationHandlers.deletions = map[PathPattern]*InoxFunction{}
				dict.ForEachEntry(ctx, func(keyRepr string, key, v Serializable) error {
					migrationHandlers.deletions[key.(PathPattern)] = v.(*InoxFunction)
					return nil
				})
			case symbolic.DB_MIGRATION__REPLACEMENTS_PROP_NAME:
				migrationHandlers.replacements = map[PathPattern]*InoxFunction{}
				dict.ForEachEntry(ctx, func(keyRepr string, key, v Serializable) error {
					migrationHandlers.replacements[key.(PathPattern)] = v.(*InoxFunction)
					return nil
				})
			case symbolic.DB_MIGRATION__INCLUSIONS_PROP_NAME:
				migrationHandlers.inclusions = map[PathPattern]*InoxFunction{}
				dict.ForEachEntry(ctx, func(keyRepr string, key, v Serializable) error {
					migrationHandlers.inclusions[key.(PathPattern)] = v.(*InoxFunction)
					return nil
				})
			case symbolic.DB_MIGRATION__INITIALIZATIONS_PROP_NAME:
				migrationHandlers.initializations = map[PathPattern]*InoxFunction{}
				dict.ForEachEntry(ctx, func(keyRepr string, key, v Serializable) error {
					migrationHandlers.initializations[key.(PathPattern)] = v.(*InoxFunction)
					return nil
				})
			default:
				return fmt.Errorf("unexpected property '%s' in migration object", k)
			}
			return nil
		})
	}

	db.inner.UpdateSchema(ctx, nextSchema, migrationHandlers)
	db.topLevelEntities = db.inner.TopLevelEntities(ctx)
}

func (db *DatabaseIL) Close(ctx *Context) error {
	if db.ownerState == nil {
		panic(ErrOwnerStateNotSet)
	}
	return db.inner.Close(ctx)
}

func (db *DatabaseIL) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "update_schema":
		return WrapGoMethod(db.UpdateSchema), true
	case "close":
		return WrapGoMethod(db.Close), true
	}
	return nil, false
}

func (db *DatabaseIL) Prop(ctx *Context, name string) Value {
	if db.ownerState == nil {
		panic(ErrOwnerStateNotSet)
	}

	switch name {
	case "schema":
		return db.initialSchema
	case "update_schema", "close":
	default:
		if db.schemaUpdateExpected {
			if !db.schemaUpdated.Load() {
				panic(ErrInvalidAccessSchemaNotUpdatedYet)
			}
		}

		val, ok := db.topLevelEntities[name]
		if ok {
			return val
		}
	}

	method, ok := db.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, db))
	}
	return method
}

func (*DatabaseIL) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (db *DatabaseIL) PropertyNames(ctx *Context) []string {
	return db.propertyNames
}

type FailedToOpenDatabase struct {
	resource SchemeHolder
}

func NewFailedToOpenDatabase(resource SchemeHolder) *FailedToOpenDatabase {
	return &FailedToOpenDatabase{resource: resource}
}

func (db *FailedToOpenDatabase) Resource() SchemeHolder {
	return db.resource
}

func (db *FailedToOpenDatabase) Schema() *ObjectPattern {
	return EMPTY_INEXACT_OBJECT_PATTERN
}

func (db *FailedToOpenDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern, handlers MigrationHandlers) {
	panic(ErrNotImplemented)
}

func (db *FailedToOpenDatabase) TopLevelEntities(_ *Context) map[string]Serializable {
	return nil
}

func (db *FailedToOpenDatabase) Close(ctx *Context) error {
	return ErrNotImplemented
}

type dummyDatabase struct {
	resource         SchemeHolder
	schemaUpdated    bool
	topLevelEntities map[string]Serializable
}

func (db *dummyDatabase) Resource() SchemeHolder {
	return db.resource
}

func (db *dummyDatabase) Schema() *ObjectPattern {
	return EMPTY_INEXACT_OBJECT_PATTERN
}

func (db *dummyDatabase) UpdateSchema(ctx *Context, schema *ObjectPattern, handlers MigrationHandlers) {
	if db.schemaUpdated {
		panic(errors.New("schema already updated"))
	}
	db.schemaUpdated = true

	state := ctx.GetClosestState()

	if len(handlers.deletions)+len(handlers.initializations)+len(handlers.replacements) > 0 {
		panic(errors.New("only inclusion handlers are supported"))
	}

	for pattern, handler := range handlers.inclusions {
		if strings.Count(string(pattern), "/") != 1 {
			panic(errors.New("only shallow inclusion handlers are supported"))
		}
		result := utils.Must(handler.Call(state, nil, []Value{}, nil))
		db.topLevelEntities[string(pattern[1:])] = result.(Serializable)
	}
}

func (db *dummyDatabase) TopLevelEntities(_ *Context) map[string]Serializable {
	return db.topLevelEntities
}

func (db *dummyDatabase) Close(ctx *Context) error {
	return ErrNotImplemented
}
