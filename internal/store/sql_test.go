package store

import (
	"context"
	"testing"
	"time"
)

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
		min, max := "MIN(a, b)", "MAX(a, b)"
		if tt.dialect == Postgres {
			min, max = "LEAST(a, b)", "GREATEST(a, b)"
		}
		if got := ScalarMin(db, "a", "b"); got != min {
			t.Errorf("%s scalar min = %q", tt.dialect, got)
		}
		if got := ScalarMax(db, "a", "b"); got != max {
			t.Errorf("%s scalar max = %q", tt.dialect, got)
		}
	}
}

func TestSQLiteUTCNowText(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	checkUTCNowText(t, context.Background(), st.DB())
}

func TestPostgresUTCNowText(t *testing.T) {
	ctx, db, _, _ := postgresTestPool(t)
	checkUTCNowText(t, ctx, db)
}

func checkUTCNowText(t *testing.T, ctx context.Context, db Execer) {
	t.Helper()
	for modifier, offset := range map[string]time.Duration{
		"-1 day":       -24 * time.Hour,
		"-6 hours":     -6 * time.Hour,
		"+300 seconds": 300 * time.Second,
	} {
		var nowText, shiftedText string
		if err := db.QueryRowContext(ctx, `SELECT `+UTCNowText(db, "")+`, `+UTCNowText(db, "?"), modifier).Scan(&nowText, &shiftedText); err != nil {
			t.Fatal(err)
		}
		now, err := time.Parse("2006-01-02 15:04:05", nowText)
		if err != nil {
			t.Fatal(err)
		}
		shifted, err := time.Parse("2006-01-02 15:04:05", shiftedText)
		if err != nil {
			t.Fatal(err)
		}
		if got := shifted.Sub(now); got != offset {
			t.Errorf("%s offset = %s, want %s", modifier, got, offset)
		}
		if age := time.Since(now); age < -time.Second || age > 10*time.Second {
			t.Errorf("database clock is not current UTC: %s", nowText)
		}
	}
}
