# Developing Arrmada

The backend is Go 1.26 with SQLite; the web UI is React, TypeScript and Tailwind (Vite),
embedded into the Go binary.

```sh
go run ./cmd/arrmada                  # backend (Go 1.26+) on :7878
cd web && npm install && npm run dev  # UI (Node 24) on :5173, proxies /api
```

For a single binary with the UI inside it:

```sh
cd web && npm run build && cd .. && go build -o arrmada ./cmd/arrmada
```

To build the Docker image from your checkout, put `ARRMADA_APP_IMAGE=arrmada:dev` in `.env`
(otherwise the build is tagged with the published image's name) and run
`docker compose up -d --build`. Without `--build`, `docker compose up -d` runs the published
image. A cold image build needs about 2 GB of free memory (4 GB is comfortable): the heavy
compiles take turns and size their parallel jobs to free memory (`docker/build-heavy`; set the
build argument `ARRMADA_BUILD_JOBS` to fix the count). A cold build takes roughly 20 minutes on
four cores; rebuilds after a Go or web change take under a minute.

## Checks

CI runs these on every push:

```sh
gofmt -l $(git ls-files '*.go')        # must print nothing
go vet ./... && go test -race -timeout 40m ./...
shellcheck -s sh update.sh install.sh docker/entrypoint.sh
cd web && npm run lint && npm test && npx tsc --noEmit -p . && npm run build && npm run e2e
```

`npm run e2e` runs the Playwright tests against the built UI with every API call answered
from fixtures in `web/e2e`, so no backend is needed.

### PostgreSQL integration tests

The PostgreSQL connection boundary is tested against a real server, not a SQL mock.
It is groundwork for the optional backend; the application still uses SQLite until
the repository queries, schema lifecycle and administration paths are ported.

Use a **disposable local database**, never an existing installation or a shared server:

```sh
export ARRMADA_TEST_POSTGRES_ADMIN_URL='postgres://postgres:test-password@127.0.0.1:5432/arrmada_test?sslmode=disable'
go test -race -count=1 ./internal/store -run 'Test(Postgres|Bind|DatabaseSQLiteBoundary)'
```

The administrator URL is for test setup only: the fixture creates randomly named schemas
and restricted login roles, runs the application connection as those roles, then removes
only its own objects. The roles cannot create databases or roles and are not superusers.
The tests check per-connection schema isolation, ownership checks, parameter binding,
prepared queries and transaction rollback while another schema holds unrelated data.
They also exercise the settings service's persistence, concurrent writes and failed-write
behavior against PostgreSQL.
Without the environment variable the real-server tests explicitly skip; CI supplies a
disposable PostgreSQL 17 service so they must run there. This does not yet establish
application-level PostgreSQL feature parity or a supported server-version range.

## Screenshots

The pictures in the README and on the website (`docs/images/*.png`) are taken from those same
fixtures — never a real server:

```sh
cd web && npm run build
README_SHOTS=1 E2E_NO_BUILD=1 npx playwright test --project=readme-shots --project=readme-shots-phone
```

`web/e2e/readme-shots.spec.ts` lists the pages; posters are made-up gradient landscapes.

## Testing the installer

`install.sh` and `update.sh` take `ARRMADA_REPO_URL`, `ARRMADA_BRANCH`, `ARRMADA_API_URL` and
`ARRMADA_TARBALL_BASE` so a fork (or a test) can point them at its own copy, and `--image` to
run any image. Test them in a throwaway `docker:dind` container, never on a real server.
