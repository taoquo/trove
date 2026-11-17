package sqlite

import (
	"database/sql/driver"
	"strings"

	"modernc.org/sqlite"
)

// trove_fold is SQLite's lower() with Unicode case folding.
//
// SQLite's built-in lower() folds ASCII only, so the search compared a query
// folded in Go with a column folded to itself: "café" found "café" but "CAFÉ"
// did not, and the same held for Cyrillic, Greek and Turkish. Folding both
// sides with a Unicode-aware rule restores the correct behaviour without a
// generated column or an index: a leading-wildcard LIKE scans the table either
// way.
const foldFunction = "trove_fold"

// foldedAuthor folds a handle in Go, the way the schema's generated column does.
//
// The author condition compares against author_handle_folded, which is trove_fold of
// the column, so the value that goes into the statement has to be folded the same
// way. strings.ToLower is the same function the SQL registration calls, which is
// what keeps the two sides agreeing.
func foldedAuthor(handle string) string { return strings.ToLower(handle) }

func init() {
	// Registration applies to connections opened afterwards, and package init
	// runs before Open can be called from another package.
	err := sqlite.RegisterDeterministicScalarFunction(foldFunction, 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) != 1 {
				return nil, nil
			}
			switch value := args[0].(type) {
			case string:
				return strings.ToLower(value), nil
			case []byte:
				return strings.ToLower(string(value)), nil
			case nil:
				return nil, nil
			default:
				return value, nil
			}
		})
	if err != nil {
		panic("sqlite: register " + foldFunction + ": " + err.Error())
	}
}
