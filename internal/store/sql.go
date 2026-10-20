package store

import (
	"context"
	"strings"
)

// InsertID inserts a row with an integer id primary key. It preserves SQLite's
// existing insert behavior and explicitly requests PostgreSQL's RETURNING id;
// it never guesses an id from a different connection or a pooled sequence.
// query is trusted INSERT SQL without a RETURNING clause.
func InsertID(ctx context.Context, q Execer, query string, args ...any) (int64, error) {
	if DialectOf(q) == Postgres {
		var id int64
		query = strings.TrimSuffix(strings.TrimSpace(query), ";") + " RETURNING id"
		err := q.QueryRowContext(ctx, query, args...).Scan(&id)
		return id, err
	}
	res, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// TimestampText gives repositories the UTC second-resolution text their APIs
// already expose, independent of a driver's TIMESTAMP scan representation.
// expression must be a trusted SQL expression, never a request value.
func TimestampText(q Execer, expression string) string {
	if DialectOf(q) == Postgres {
		return "to_char(" + expression + ", 'YYYY-MM-DD HH24:MI:SS')"
	}
	return "strftime('%Y-%m-%d %H:%M:%S', " + expression + ")"
}

// FoldASCII retains SQLite's username/case-folding semantics regardless of the
// PostgreSQL database's locale. Like TimestampText, expression is trusted SQL.
func FoldASCII(q Execer, expression string) string {
	if DialectOf(q) == Postgres {
		return "ascii_lower(" + expression + ")"
	}
	return "lower(" + expression + ")"
}

// UTCNowText is the database clock in SQLite's UTC second-resolution layout,
// for columns that deliberately store dates as text. An optional modifier is
// a trusted SQL expression (usually a bound ? interval), never interpolated input.
// Calendar month/year modifiers are not portable and must not be passed here.
func UTCNowText(q Execer, modifier string) string {
	if DialectOf(q) == Postgres {
		return "to_char(" + UTCNowTimestamp(q, modifier) + ", 'YYYY-MM-DD HH24:MI:SS')"
	}
	if modifier != "" {
		return "datetime('now', " + modifier + ")"
	}
	return "datetime('now')"
}

// UTCNowTimestamp is the native UTC clock for TIMESTAMP columns. SQLite has no
// separate timestamp type, so there it uses the same text representation.
// The modifier contract is the same as UTCNowText.
func UTCNowTimestamp(q Execer, modifier string) string {
	if DialectOf(q) != Postgres {
		return UTCNowText(q, modifier)
	}
	expression := "statement_timestamp() AT TIME ZONE 'UTC'"
	if modifier != "" {
		expression = "(" + expression + ") + CAST(" + modifier + " AS interval)"
	}
	return expression
}
