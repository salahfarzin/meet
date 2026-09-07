# Commands

```bash
make dev               # hot-reload dev server (air)
make build && make run # compile to bin/ then run
make test              # unit tests
make watch             # tests in watch mode
make lint              # golangci-lint
make lint-fix          # golangci-lint --fix
make test-coverage     # coverage report (CI gate: min 95%)
make quality-check     # lint + test-coverage + security-scan
make quality-gate      # full aggregated gate (run before PR)
make security-scan     # gosec
make complexity-check  # cyclomatic complexity (max 10)
```

Run a single Go test:

```bash
go test ./internal/meets/ -run TestCreate -v
go test ./internal/meets/ -run TestService_Create/conflict -v   # subtest
```

## Proto & migrations

```bash
make generate-proto    # regen Go + grpc-gateway stubs from proto/
make migrate           # apply up migrations (golang-migrate, MySQL)
make migrate-down      # roll back
make migrate-create    # scaffold a new migration pair
```

Raw proto regen (needs googleapis on include path):

```bash
protoc -I. -I./googleapis --go_out=proto --go-grpc_out=proto --grpc-gateway_out=proto proto/meets/meets.proto
```

Pre-commit hooks: `git config core.hooksPath .githooks` (runs quality checks before commit).
