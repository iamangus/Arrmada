# PostgreSQL migrations

This is a separate migration history from `../migrations` (SQLite). Never edit or
remove a migration after it ships. PostgreSQL migrations run in transactions and
must not contain `BEGIN` or `COMMIT`.

`0001_baseline.sql` is generated from a new SQLite installation at
`0347_quality_multi_counts_language`, not by replaying SQLite DDL on PostgreSQL:

```sh
go run ./scripts/postgres-baseline -postgres
```

The generated baseline preserves columns, defaults, primary/unique keys, foreign
keys, check constraints, indexes and seed data. Integers remain signed 64-bit;
floating values remain double precision. Flags and JSON retain their existing
integer/text representations. Foreign keys are initially immediate but deferrable,
so offline conversion can validate a complete dataset with cyclic relationships.
Text uses the `C` collation to retain SQLite's byte ordering and equality. The ASCII
fold helper preserves SQLite's case folding without requiring extensions.

Behavioral triggers are reviewed and maintained separately in `0002_behavior.sql`.
They must preserve book edits, audiobook metadata invalidation and viewing-group
maintenance. Table/column parity alone does not prove behavioral parity.
