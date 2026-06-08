# API Server

Placeholder folder for the Go + Gin backend.

The backend bootstrap phase will add:

- `go.mod`
- `cmd/server/main.go`
- `cmd/migrate/main.go`
- `internal/config`
- `internal/db`
- `internal/httpserver`
- `migrations`

## Local Commands

Run migration from host after PostgreSQL is available:

```bash
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/migrate down
```

Run API:

```bash
go run ./cmd/server
```
