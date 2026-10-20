package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// PostgresOptions describes access to a pre-provisioned schema, not permission
// to administer its database. URL is secret and must never be logged.
type PostgresOptions struct {
	URL            string
	Schema         string
	MaxConnections int
}

var postgresSchemaName = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

func (o PostgresOptions) validate() error {
	if !postgresSchemaName.MatchString(o.Schema) || o.Schema == "public" ||
		o.Schema == "information_schema" || strings.HasPrefix(o.Schema, "pg_") {
		return errors.New("PostgreSQL requires a dedicated schema with a lowercase SQL identifier (not public, information_schema or pg_*)")
	}
	// Requiring an explicit URL avoids accidentally connecting to a database
	// selected by libpq environment defaults. Never echo a malformed secret.
	u, err := url.Parse(o.URL)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") ||
		u.Hostname() == "" || u.User == nil || u.User.Username() == "" || strings.Trim(u.Path, "/") == "" || u.Fragment != "" {
		return errors.New("PostgreSQL requires a postgres:// URL with an explicit host, user and database")
	}
	if o.MaxConnections < 0 {
		return errors.New("PostgreSQL maximum connections must not be negative")
	}
	return nil
}

// openPostgresPool establishes the connection boundary only. Schema migrations
// belong to Store's lifecycle, not to a connector or individual repository.
// In particular this never creates a schema or falls back to SQLite.
func openPostgresPool(ctx context.Context, opt PostgresOptions) (*Database, error) {
	if err := opt.validate(); err != nil {
		return nil, err
	}
	cfg, err := pgx.ParseConfig(opt.URL)
	if err != nil {
		// pgx's parse error contains the connection string, including passwords.
		return nil, errors.New("invalid PostgreSQL connection options")
	}
	// Explicitly exclude public, $user and implicit pg_temp precedence. Schemas
	// are provisioned by the operator and ownership is checked below. UTC keeps
	// timestamp behavior independent of the shared server's session defaults.
	cfg.RuntimeParams["search_path"] = pgx.Identifier{opt.Schema}.Sanitize() + ", pg_catalog, pg_temp"
	cfg.RuntimeParams["timezone"] = "UTC"
	cfg.RuntimeParams["standard_conforming_strings"] = "on"
	cfg.RuntimeParams["default_transaction_isolation"] = "read committed"
	cfg.RuntimeParams["application_name"] = "arrmada"
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	pool := stdlib.OpenDB(*cfg)
	maxConns := opt.MaxConnections
	if maxConns == 0 {
		maxConns = 4
	}
	pool.SetMaxOpenConns(maxConns)
	pool.SetMaxIdleConns(maxConns)
	pool.SetConnMaxLifetime(time.Hour)
	db := &Database{pool: pool, dialect: Postgres}
	if err := pool.PingContext(ctx); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("connect PostgreSQL: %w", err)
	}
	var owns, usable bool
	err = db.QueryRowContext(ctx, `SELECT
		pg_has_role(current_user, n.nspowner, 'USAGE'),
		has_schema_privilege(current_user, n.oid, 'USAGE') AND has_schema_privilege(current_user, n.oid, 'CREATE')
		FROM pg_catalog.pg_namespace AS n WHERE n.nspname = ?`, opt.Schema).Scan(&owns, &usable)
	if err != nil || !owns || !usable {
		_ = pool.Close()
		return nil, errors.New("PostgreSQL schema must already exist and be owned by the application role, with USAGE and CREATE privileges")
	}
	return db, nil
}
