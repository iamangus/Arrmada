package store_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/tristenlammi/arrmada/internal/settings"
	"github.com/tristenlammi/arrmada/internal/store"
)

// Exercise a real repository, not just the driver's SQL support. The settings
// service keeps its existing SQL and memory/write-through guarantees on both
// backends; it only needs the shared query boundary instead of a concrete pool.
func TestPostgresSettingsPersistenceAndFailedWrites(t *testing.T) {
	ctx, _, opt, _ := store.PostgresFixture(t)
	db, err := store.OpenPostgresPool(ctx, opt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := settings.Open(ctx, db); err == nil {
		t.Fatal("opened settings without a readable table")
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE settings (
		key text PRIMARY KEY, value text NOT NULL,
		updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}
	svc, err := settings.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	key := "literal ? key'; --"
	if err := svc.Set(ctx, key, "original"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set(ctx, key, "updated"); err != nil {
		t.Fatal(err)
	}
	reopened, err := settings.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Get(ctx, key, "missing"); got != "updated" {
		t.Fatalf("persisted value = %q", got)
	}

	var writers sync.WaitGroup
	for i := range 20 {
		writers.Go(func() {
			if err := svc.Set(ctx, key, fmt.Sprint(i)); err != nil {
				t.Errorf("concurrent settings write: %v", err)
			}
		})
	}
	writers.Wait()
	var persisted string
	if err := db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if got := svc.Get(ctx, key, "missing"); got != persisted {
		t.Fatalf("memory %q differs from database %q", got, persisted)
	}
	// No failed database operation may replace the last successfully loaded
	// settings in memory with defaults or an unsaved value.
	if _, err := db.ExecContext(ctx, `DROP TABLE settings`); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set(ctx, key, "not saved"); err == nil {
		t.Fatal("write to missing settings table succeeded")
	}
	if err := svc.Reload(ctx); err == nil {
		t.Fatal("reload of missing settings table succeeded")
	}
	if got := svc.Get(ctx, key, "missing"); got != persisted {
		t.Fatalf("failed write/reload changed memory to %q", got)
	}
}
