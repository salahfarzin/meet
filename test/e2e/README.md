# End-to-End (E2E) Tests

This directory contains end-to-end tests for the Meets API. These tests verify the complete request/response flow through the HTTP API endpoints.

## Overview

The E2E tests cover:
- ✅ **Create Meet** - POST `/api/v1/meets`
- ✅ **Get One Meet** - GET `/api/v1/meets/{uuid}`
- ✅ **Get All Meets** - GET `/api/v1/meets`
- ✅ **Update Meet** - PUT `/api/v1/meets/{uuid}`
- ✅ **Delete Meet** - DELETE `/api/v1/meets/{uuid}`
- ✅ **Full CRUD Flow** - Complete create → read → update → delete workflow

## Prerequisites

1. **Database**: MySQL must be running with migrations applied
2. **Application**: The meets API server must be running
3. **Environment**: Set `APP_URL` environment variable (optional, defaults to `http://localhost:8080`)

## Running the Tests

### Option 1: Manual (Application Already Running)

If you have the application running locally:

```bash
# Ensure the app is running
make watch

# In another terminal, run E2E tests
APP_URL=http://localhost:8080 go test -v ./test/e2e/...
```

### Option 2: Automated (Start App, Run Tests, Stop App)

```bash
# Start the application in the background
./bin/meet-api &
APP_PID=$!

# Wait for server to be ready
sleep 5

# Run E2E tests
APP_URL=http://localhost:8080 go test -v ./test/e2e/...

# Stop the application
kill $APP_PID
```

### Option 3: Using Docker Compose

```bash
# Start all services (app + database)
docker-compose up -d

# Run E2E tests
APP_URL=http://localhost:8080 go test -v ./test/e2e/...

# Stop services
docker-compose down
```

## Test Structure

### `helpers.go`
Contains utility functions for E2E testing:
- `TestConfig` - Configuration for test execution
- `DoRequest()` - HTTP request helper
- `GetAuthHeaders()` - Mock authentication headers
- `WaitForServer()` - Server readiness check

### `meets_api_test.go`
Contains all E2E test cases:
- `TestMeetsAPI_CreateMeet` - Tests meet creation with various scenarios
- `TestMeetsAPI_GetOneMeet` - Tests retrieving a single meet
- `TestMeetsAPI_GetAllMeets` - Tests listing meets with filters
- `TestMeetsAPI_UpdateMeet` - Tests updating meets
- `TestMeetsAPI_DeleteMeet` - Tests deleting meets
- `TestMeetsAPI_FullCRUDFlow` - Tests complete CRUD workflow

## Authentication

The tests use mock authentication headers for simplicity:
```go
X-User: test-user
X-User-Uuid: test-user-uuid-123
X-User-Roles: Programmer
```

For production E2E tests, you would:
1. Call the auth service to get a real token
2. Use that token in the `Authorization: Bearer <token>` header

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_URL` | Base URL of the API server | `http://localhost:8080` |
| `DB_HOST` | Database host | `127.0.0.1` |
| `DB_PORT` | Database port | `3306` |
| `DB_USER` | Database user | `meet_user` |
| `DB_PASSWORD` | Database password | `root` |
| `DB_NAME` | Database name | `meet_db_test` |

## CI/CD Integration

The E2E tests are integrated into the GitHub Actions workflow:

```yaml
- name: Run E2E tests
  run: |
    if [ -d "test/e2e" ] && [ "$(ls -A test/e2e 2>/dev/null)" ]; then
      go test -v ./test/e2e/...
    else
      echo "⚠️  No E2E tests found, skipping..."
    fi
  env:
    APP_URL: http://localhost:8080
```

## Test Data Cleanup

⚠️ **Note**: These tests create real data in the database. For a clean test environment:

1. Use a separate test database (`meet_db_test`)
2. Run migrations before tests
3. Optionally truncate tables between test runs

## Troubleshooting

### Server Not Ready
```
Error: server not ready after 10 retries
```
**Solution**: Ensure the application is running and accessible at `APP_URL`

### Connection Refused
```
Error: dial tcp 127.0.0.1:8080: connect: connection refused
```
**Solution**: Start the application before running tests

### Authentication Errors
```
Error: 401 Unauthorized
```
**Solution**: Check that `GetAuthHeaders()` is being used in requests

## Best Practices

1. **Isolation**: Each test should be independent
2. **Cleanup**: Tests create data but don't clean up (use test database)
3. **Assertions**: Use descriptive error messages
4. **Logging**: Use `t.Log()` for debugging
5. **Timeouts**: HTTP client has 10s timeout

## Example Test Run

```bash
$ go test -v ./test/e2e/...

=== RUN   TestMeetsAPI_CreateMeet
=== RUN   TestMeetsAPI_CreateMeet/Create_meet_successfully
=== RUN   TestMeetsAPI_CreateMeet/Create_meet_with_missing_title
=== RUN   TestMeetsAPI_CreateMeet/Create_meet_with_missing_start_time
--- PASS: TestMeetsAPI_CreateMeet (0.15s)
    --- PASS: TestMeetsAPI_CreateMeet/Create_meet_successfully (0.05s)
    --- PASS: TestMeetsAPI_CreateMeet/Create_meet_with_missing_title (0.05s)
    --- PASS: TestMeetsAPI_CreateMeet/Create_meet_with_missing_start_time (0.05s)

=== RUN   TestMeetsAPI_FullCRUDFlow
    meets_api_test.go:XXX: Step 1: Creating a meet
    meets_api_test.go:XXX: Created meet with UUID: abc-123
    meets_api_test.go:XXX: Step 2: Reading the created meet
    meets_api_test.go:XXX: Step 3: Updating the meet
    meets_api_test.go:XXX: Step 4: Verifying the update
    meets_api_test.go:XXX: Step 5: Deleting the meet
    meets_api_test.go:XXX: Step 6: Verifying deletion
    meets_api_test.go:XXX: ✅ Full CRUD flow completed successfully
--- PASS: TestMeetsAPI_FullCRUDFlow (0.30s)

PASS
ok      github.com/salahfarzin/meet/test/e2e    1.234s
```

## Next Steps

- Add tests for `/meets/{uuid}/availability` endpoint
- Add tests for `/meets/{uuid}/types` endpoint
- Add performance/load tests
- Add tests for concurrent operations
- Add tests for edge cases (timezone handling, etc.)
