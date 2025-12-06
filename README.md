# Meet System Backend

This project is a backend system designed for managing meets for psychologists and doctors using gRPC and RabbitMQ.

## Features

- gRPC-based API for handling meet requests.
- Separate services for managing doctors and psychologists.
- Integration with RabbitMQ for message queuing.

## REST API
```
curl http://localhost:8080/meets
```
## GRPC API
```
grpcurl -plaintext -d '{"meet": {"title":"Test","start":"2025-09-10 10:25"}}' localhost:50051 meets.MeetService/CreateMeet
```

## Build proto for meet (with support RESTful API)
```
protoc -I. -I./googleapis --go_out=proto --go-grpc_out=proto --grpc-gateway_out=proto proto/meets/meets.proto 
```

## Database migrations
the most popular tool for generating and running database migrations is golang-migrate/migrate.

Migration path: ```migrations```
### Install migrate CLI (if you haven’t already):
```
brew install golang-migrate
```
### Create migrations
```
migrate create -ext sql -dir migrations -seq create_meets_table
```
### Run migration
```
migrate -path migrations -database \"mysql://user:password@tcp(localhost:3306)/dbname\" up
```

## Quality Assurance & CI/CD

This project implements comprehensive quality pipelines to ensure code reliability, security, and maintainability.

### Quality Gates

The project enforces the following quality standards:

- **Test Coverage**: Minimum 95% for CI builds and PRs
- **Linting**: Zero critical issues with golangci-lint
- **Security**: Automated vulnerability scanning with gosec and Trivy
- **Code Complexity**: Maximum cyclomatic complexity of 10
- **Dependencies**: Automated dependency review for vulnerabilities and license compliance

### Local Quality Checks

Run all quality checks locally:

```bash
make quality-check
# or
./scripts/quality-gate.sh
```

Individual checks:

```bash
make lint              # Run golangci-lint
make lint-fix          # Auto-fix linting issues
make test-coverage     # Run tests with coverage report
make security-scan     # Run security vulnerability scan
make complexity-check  # Check code complexity
make vuln-check        # Check for known vulnerabilities
```

### CI/CD Pipelines

#### Main CI Pipeline (`.github/workflows/ci.yml`)
- **Lint**: Code quality checks with golangci-lint
- **Test**: Unit tests with race detection and coverage analysis
- **Security Scan**: gosec security vulnerability scanning
- **Vulnerability Scan**: Trivy container vulnerability scanning
- **Quality Gate**: Aggregates all quality checks
- **Build**: Compiles the application (only after quality gate passes)
- **Benchmark**: Performance benchmarking with historical tracking
- **Code Quality Metrics**: Generates complexity and maintainability reports
- **Mutation Testing**: Validates test suite effectiveness

#### PR Checks (`.github/workflows/pr-checks.yml`)
- **Conventional Commits**: Validates PR title format
- **PR Labeling**: Auto-labels PRs based on changed files
- **PR Quality Gate**: Ensures PRs meet quality standards

#### Integration Tests (`.github/workflows/integration-tests.yml`)
- **Full Stack Testing**: End-to-end tests with real database and API
- **Scheduled Runs**: Daily integration test runs
- **Manual Trigger**: Can be run on-demand

#### Other Workflows
- **CD**: Docker build and deployment pipeline
- **Release**: Automated releases with semantic versioning
- **Dependency Review**: Security and license checks for dependencies

### Pre-commit Hooks

Install pre-commit hooks to run quality checks before commits:

```bash
git config core.hooksPath .githooks
```

This will run quality checks automatically before each commit.

### Code Quality Metrics

The CI pipeline generates several quality metrics:

- **Test Coverage**: Tracks code coverage over time
- **Performance Benchmarks**: Monitors performance regressions
- **Code Complexity**: Identifies functions that need refactoring
- **Security Issues**: Automated detection of security vulnerabilities
- **Technical Debt**: TODO/FIXME comments tracking

### Best Practices Enforced

1. **Testing**:
   - Unit tests for all business logic
   - Integration tests for full stack validation
   - Race condition detection in concurrent code
   - Minimum coverage thresholds

2. **Code Quality**:
   - Consistent formatting with gofmt
   - Static analysis with comprehensive linters
   - Complexity limits to maintain readability
   - Security best practices enforcement

3. **Security**:
   - Automated vulnerability scanning
   - Dependency security reviews
   - Secure coding practices validation
   - Container security scanning

4. **Performance**:
   - Benchmark tracking to detect regressions
   - Memory usage monitoring
   - Concurrent execution testing

5. **Maintainability**:
   - Code complexity analysis
   - Technical debt tracking
   - Automated refactoring suggestions