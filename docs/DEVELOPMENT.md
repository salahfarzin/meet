# Developer Guide

This guide describes the workflow for developing, testing, and maintaining the Meet service.

## Setup
Ensure you have the following installed:
- Go 1.21+
- [buf](https://buf.build/docs/installation) (for Protobuf management)
- [migrate](https://github.com/golang-migrate/migrate) (for database migrations)
- [air](https://github.com/cosmtrek/air) (for live reload)

## Workflow

### 1. Modifying API (Proto)
When you change `.proto` files in `proto/meets/`:
1. Run `make generate-proto`.
2. This generates:
   - Go gRPC stubs
   - REST gateway stubs
   - OpenAPI documentation (`openapi.yaml`)
   - Embedded assets in `pkg/swagger`

### 2. Database Migrations
We use `golang-migrate`.
- **Create**: `make migrate-create name=my_new_migration`
- **Apply Up**: `make migrate`
- **Rollback**: `make migrate-down`

### 3. Running Locally
- Use `make watch` to start the server with live-reloading (Air).
- The gRPC server runs on port `:50052`.
- The REST server runs on port `:8080`.

### 4. API Documentation
- Access the interactive docs at `http://localhost:8080/docs/`.
- The source for these docs is generated into `pkg/swagger/gen/`.

### 5. Testing
- **Run all tests**: `make test`
- **Check coverage**: `make test-coverage`
- Minimum required coverage is **95%**.

## Coding Standards
- **Errors**: Return meaningful gRPC status codes (e.g., `codes.InvalidArgument`, `codes.Internal`).
- **Dates**: Always use RFC3339 strings in APIs and `time.Time` in internal logic.
- **Logging**: Use the `logger` package with `zap`. Always include relevant context.
- **Linters**: Run `make lint` before pushing. We use `golangci-lint`.
