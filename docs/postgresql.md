# PostgreSQL (opt-in)

SQLite remains the default. PostgreSQL is an alternative for **one Arrmada
instance**, not a multi-replica mode. Supported servers are PostgreSQL **16, 17
and 18**, in an existing **UTF8** database. Arrmada uses a dedicated schema; it
never creates a database, installs extensions, or administers unrelated schemas.

This branch is still under development: runtime selection, native backups and
offline PostgreSQL recovery are implemented; **SQLite ↔ PostgreSQL conversion
is not yet implemented**. Do not switch an existing installation's backend
expecting its data to be copied. Keep its original SQLite database intact.

## Provisioning and privileges

Ask the database operator to provision a login and an empty schema in the
existing database. For example, run by the operator (not Arrmada):

```sql
CREATE ROLE arrmada_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT
  PASSWORD 'a-secret-set-by-the-operator';
GRANT CONNECT ON DATABASE existing_db TO arrmada_app;
-- Connected to existing_db:
CREATE SCHEMA arrmada AUTHORIZATION arrmada_app;
```

Use a lowercase schema identifier, up to 63 characters. `public`,
`information_schema`, and `pg_*` are refused. The application role must own the
schema (and objects Arrmada creates within it), with USAGE and CREATE. Ownership
also permits its trusted migrations and recovery to alter its own tables,
sequences, functions and user triggers. No superuser, database-creation,
replication, extension, or other application's object privileges are needed.
The operator remains responsible for the shared database's role/public-schema
policies; Arrmada does not change those policies.

The schema must initially be empty. Existing arbitrary tables/functions are not
adopted. A second Arrmada instance targeting the same database/schema is refused.
Different designated schemas are independent. Runtime connection sessions hold
schema-scoped locks so recovery/upgrades cannot overlap surviving old sessions
after a coordinator connection is lost. Arrmada shuts down rather than silently
rejoining as another instance.
After a network partition, server-side sessions can retain locks until PostgreSQL
detects their disconnection. Startup/recovery refuses to overlap them. Configure
appropriate connection/TCP timeouts with the database operator; if manual session
cleanup is necessary, first establish that the old process cannot resume. Arrmada
never terminates another session automatically to force ownership.

## Configuration

Set these in the app's private environment or Compose `.env` (mode `0600`):

```dotenv
ARRMADA_DATABASE_BACKEND=postgres
ARRMADA_POSTGRES_URL=postgres://arrmada_app:URL_ENCODED_PASSWORD@db.example/existing_db?sslmode=verify-full&sslrootcert=/certs/ca.pem
ARRMADA_POSTGRES_SCHEMA=arrmada
ARRMADA_POSTGRES_MAX_CONNECTIONS=4
```

URL-encode credentials. Never pass the URL as a command-line argument, publish
`docker compose config` output containing it, or put it in logs. For remote
connections use certificate-verified TLS (`sslmode=verify-full`), with the CA
and any client certificate/key files readable by the application user and mounted
at the same paths for Go and native clients. libpq's `prefer` default can fall
back to plaintext; do not rely on it for an untrusted network.

The backend defaults to `sqlite`; a URL alone never changes it. Invalid backend
names, connection/ownership failures and failed migrations stop startup, never
fall back to SQLite. `arrmada.db` is neither opened nor automatically converted
when PostgreSQL is selected. Resolve any previously staged SQLite restore
explicitly before switching backends. Revert the selector to `sqlite` to use the
old file again, understanding that changes made in PostgreSQL are not in it.

The pool limit is 1–100 (default 4). Allow additional connections for ownership,
pre-upgrade inspection, native dumping and maintenance commands; typically the
configured application pool plus three, with additional headroom for CLI work.
The session uses Read Committed, UTC, UTF8 and ISO/YMD dates independently of
shared-server defaults. Application-local graph timezones and schedules still
use Arrmada's configured `TZ`.

The image bundles PostgreSQL 18 native clients on both architectures, including
the all-in-one variant. For a bare Go binary, install `pg_dump` and `pg_restore`
from versions 16–18, **no older than the server**, on PATH. Optionally set
`ARRMADA_POSTGRES_BIN` to their directory. Startup checks the clients before a
new installation can accumulate data. Current native backup support requires a
single-host URL, no libpq service file, and no encrypted-key `sslpassword`
option. Unsupported settings fail explicitly. The server and schema are not
added to Compose; provision and back them up according to your database policy.

## Backups and upgrades

The same Backups card, nightly schedule, manual backup, download, safety copies
and per-kind retention apply. PostgreSQL copies are **native custom archives**
named `arrmada-<kind>-<timestamp>.pgdump`, kept under `<data>/backups` on the
Arrmada server, not necessarily the database server's disk. Downloads are raw
archives, not `.db.gz`. They contain credentials and all user state: protect
them, download an off-server copy, and test recovery. Database-server backups
and point-in-time recovery remain the operator's responsibility.

Native dumps are schema-scoped, consistent, decoded completely before publication,
private on disk, and never overwrite an existing backup. If schema protection
is lost during dumping, no finished backup is published. Retention never prunes
the other backend's copies after a switch. Native archive metadata is decoded
without connecting to a database or executing archive SQL.
The backup directory must support hard links for atomic non-overwrite publication
(checked before startup). Allow room for the uncompressed archive **plus at least
17 MiB** of application headroom. COPY output can greatly exceed PostgreSQL's
compressed relation size; streaming free-space checks stop a growing dump rather
than allowing that estimate to exhaust the application's disk.

`arrmada backup`, `schema`, `reset-password` and `reset-2fa` use the selected
backend without creating/migrating it. Native backups require an initialized
Arrmada schema. Existing PostgreSQL schemas are upgraded only after a successful
native pre-migration backup. Checksums and unknown/newer migrations are checked.
The SQLite escape hatches `ARRMADA_SKIP_MIGRATION_SNAPSHOT` and
`ARRMADA_ALLOW_NEWER_SCHEMA` are deliberately **not supported** for PostgreSQL.

`update.sh` takes its normal native pre-update backup. A code-only rollback is
permitted only when the old build supports PostgreSQL and accepts the current
schema. An older SQLite-only image is refused before it can open `arrmada.db`.
`--rollback --with-db` is refused for PostgreSQL **before** containers/images or
schemas change: database recovery is an explicit offline operation below.
Switching images does not undo migrations. Keep the matching old image/binary
and archive for rollback.

## Offline restore / database rollback

There is **no in-app PostgreSQL restore or upload/staged replacement**. Use the
offline command with a build whose PostgreSQL migration history exactly matches
the archive. Recover an old backup with that matching old build first; upgrade
after recovery, not while interpreting a different schema version.

1. Stop Arrmada and keep the old database schema and filesystem data intact.
2. Have the operator provision a **new empty schema**, owned by the same
   application role. Existing/occupied destinations, even another Arrmada
   installation, are refused; they are never dropped or truncated.
3. Supply `ARRMADA_POSTGRES_URL` securely through the environment and run as the
   filesystem/application user (use the matching image for a container install):

   ```sh
   arrmada restore-postgres --schema arrmada_recovered /safe/backup.pgdump
   ```

4. After success, set `ARRMADA_POSTGRES_SCHEMA=arrmada_recovered` and start the
   matching build. For a code rollback, `update.sh --rollback` without `--with-db`
   can then select the matching PostgreSQL-capable previous image.

Arrmada preserves the source archive. `pg_restore` only decodes it into a private
temporary file: no archive DDL, functions, role/session commands or sequence SQL
are executed. Trusted embedded migrations, validated COPY data, all-table row
counts/fingerprints, deferred relationship checks, user-trigger restoration and
identity sequence state commit together. Failed transfers roll back destination
initialization as well as data; a failed destination does not have usable
migration metadata. Temporary decoded data is removed (a hard process/machine
crash can leave a private hidden temporary file for manual cleanup).
Allow room in the data directory for decoded COPY data as well as any local
source archive; streaming free-space guards retain the same application reserve
used for backups, including when a compressed native archive expands greatly.

Only Arrmada native COPY custom archives are accepted, not SQL scripts, directory
archives, SQLite files or database-wide dumps containing other schemas. Source
and destination schema names can differ. Connections lost during COMMIT have
the normal remote-transaction ambiguity: inspect the destination before retrying.
An occupied destination must never be cleared automatically. No recovery uses
`DROP ... CASCADE`, trigger-constraint bypass, or superuser privileges.

## What still lives on disk

Keep the persistent data directory and existing media/download mounts. PostgreSQL
does not move covers, media, logs, local model files, certificates, conversion
scratch/holds, bundled companion configuration or native backup archives into
the database. Back up those files separately and preserve paths/permissions.
Database recovery alone cannot recreate or undo media filesystem operations.
