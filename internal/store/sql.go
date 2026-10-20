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
