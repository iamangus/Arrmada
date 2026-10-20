package store

import "testing"

func TestSQLExpressionHelpers(t *testing.T) {
	for _, tt := range []struct {
		dialect Dialect
		time    string
		fold    string
	}{
		{SQLite, "strftime('%Y-%m-%d %H:%M:%S', s.expires_at)", "lower(?)"},
		{Postgres, "to_char(s.expires_at, 'YYYY-MM-DD HH24:MI:SS')", "ascii_lower(?)"},
	} {
		db := &Database{dialect: tt.dialect}
		if got := TimestampText(db, "s.expires_at"); got != tt.time {
			t.Errorf("%s time expression = %q", tt.dialect, got)
		}
		if got := FoldASCII(db, "?"); got != tt.fold {
			t.Errorf("%s fold expression = %q", tt.dialect, got)
		}
	}
}
