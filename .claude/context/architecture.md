# Architecture

Go gRPC microservice managing **meets** (appointments/meetings for psychologists & doctors). Single binary runs **two servers concurrently** (see `cmd/api/app.go` → `App.Serve`):

- **gRPC** server on `GRPC_PORT` (default 50052; `.env.example` uses 50051) — `cmd/api/grpc.go`
- **REST** gateway (grpc-gateway) on `APP_PORT` (default 8080) under `REST_PREFIX` (default `/api/v1`) — `cmd/api/rest.go`

The REST server is a thin reverse-proxy: it dials the local gRPC server and translates HTTP↔gRPC. Every REST route maps to a gRPC method via `google.api.http` annotations in the proto. There is no separate REST handler logic.

## Request flow

```
HTTP → REST gateway (rest.go) → gRPC (grpc.go) → internal/meets/handler.go
     → internal/meets/service.go (business rules) → internal/meets/repository.go (MySQL)
```

Layering is strict and interface-driven (`Handler` → `Service` → `Repository`), each with its own `_test.go`. Repository uses `database/sql` with `go-sqlmock` in tests.

## Boot sequence (`main.go` → `setup()`)

1. `configs.Init()` loads `.env` (godotenv) + env vars.
2. Zap logger initialized to `storage/logs/app-<date>.log`.
3. MySQL pool via `github.com/salahfarzin/utils/db` (shared internal lib).
4. `App{Configs, Logger, DB, AllowedOrigins}` → `Serve()` (graceful shutdown on SIGINT/SIGTERM).

## Auth (delegated)

This service does **not** issue or validate tokens itself. `rest.go`'s `AuthMiddleware` takes the incoming `Bearer` token and calls `AUTH_SERVICE + "/me"`; on 200 it injects the `User` (uuid, roles) into context. The gateway forwards `x-user`, `x-user-uuid`, `x-user-roles` headers as gRPC metadata.

- `APP_ENV=test` swaps in `TestAuthMiddleware` (auth disabled) for E2E.
- `retrieveOrganizerUuid` (handler.go): organizer defaults to the authenticated user's uuid; only the `Programmer` role may set an arbitrary `organizer_uuid` (admin/impersonation).

## Health & docs

Health endpoints bypass auth: `<prefix>/health`, `/live`, `/ready` (`internal/health`). Swagger UI + OpenAPI/Postman served from embedded files at `<prefix>/docs/`, `/openapi.yaml`, `/postman_collection.json`.
