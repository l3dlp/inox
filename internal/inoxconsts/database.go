package inoxconsts

import "slices"

const (
	LDB_SCHEME_NAME string = "ldb"
	ODB_SCHEME_NAME string = "odb"

	DB_MIGRATION__DELETIONS_PROP_NAME       = "deletions"
	DB_MIGRATION__INCLUSIONS_PROP_NAME      = "inclusions"
	DB_MIGRATION__REPLACEMENTS_PROP_NAME    = "replacements"
	DB_MIGRATION__INITIALIZATIONS_PROP_NAME = "initializations"
)

var (
	DB_MIGRATION_PROP_NAMES = []string{
		DB_MIGRATION__DELETIONS_PROP_NAME, DB_MIGRATION__INCLUSIONS_PROP_NAME, DB_MIGRATION__REPLACEMENTS_PROP_NAME,
		DB_MIGRATION__INITIALIZATIONS_PROP_NAME,
	}
)

func IsDbMigrationPropertyName(name string) bool {
	return slices.Contains(DB_MIGRATION_PROP_NAMES, name)
}
