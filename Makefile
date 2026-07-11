PROTO_DIR := proto
PROTO_SRC := $(wildcard $(PROTO_DIR)/*.proto)
GO_OUT := .
GOOGLEAPIS_DIR := $(HOME)/go/src/googleapis

.PHONY: generate-proto
generate-proto:
	@mkdir -p pkg/swagger/gen
	cd $(PROTO_DIR) && buf generate

.PHONY: migrate
migrate:
	@if [ ! -f .env ]; then echo "❌ .env file not found"; exit 1; fi
	@set -a; source .env; set +a; migrate -path migrations -database "mysql://meet_user:root@tcp(127.0.0.1:3306)/meet_db" up

.PHONY: migrate-down
migrate-down:
	@if [ ! -f .env ]; then echo "❌ .env file not found"; exit 1; fi
	@set -a; source .env; set +a; migrate -path migrations -database "mysql://meet_user:root@tcp(127.0.0.1:3306)/meet_db" down

.PHONY: migrate-create
migrate-create:
	@if [ -z "$(name)" ]; then echo "❌ Usage: make migrate-create name=migration_name"; exit 1; fi
	@migrate create -ext sql -dir migrations -seq $(name)

# Local Kubernetes (e.g. docker-desktop via ansible/deploy-local-k8s.yml).
# entrypoint.sh's auto-migrate only sees DATABASE_URL if it's a real shell env
# var, but it's mounted as a file (/app/.env) with no export step - these
# targets export it from that file explicitly before invoking `migrate`.
.PHONY: migrate-k8s
migrate-k8s:
	kubectl exec -n psychometrist deploy/meet -- sh -c 'export $$(grep DATABASE_URL /app/.env) && migrate -path /app/migrations -database "$$DATABASE_URL" up'

.PHONY: migrate-k8s-down
migrate-k8s-down:
	kubectl exec -n psychometrist deploy/meet -- sh -c 'export $$(grep DATABASE_URL /app/.env) && migrate -path /app/migrations -database "$$DATABASE_URL" down'

.PHONY: build
build:
	$(MAKE) generate-proto
	@go build -o bin/meet-api

.PHONY: dev
dev:
	@go run main.go

.PHONY: run
run: build
	@./bin/meet-api

.PHONY: test
test:
	@go test -v ./...

.PHONY: test-e2e
test-e2e:
	@echo "🧪 Running E2E tests..."
	@if [ -z "$(APP_URL)" ]; then \
		echo "⚠️  APP_URL not set, using default: http://localhost:8083"; \
		APP_URL=http://localhost:8083 APP_ENV=test go test -v ./test/e2e/... -count=1; \
	else \
		APP_ENV=test go test -v ./test/e2e/... -count=1; \
	fi

.PHONY: test-e2e-with-app
test-e2e-with-app:
	@echo "🚀 Starting application in TEST mode..."
	@$(MAKE) build
	@APP_ENV=test ./bin/meet-api & echo $$! > /tmp/meet-api.pid
	@echo "⏳ Waiting for server to be ready..."
	@sleep 5
	@echo "🧪 Running E2E tests..."
	@APP_URL=http://localhost:8083 APP_ENV=test go test -v ./test/e2e/... -count=1 || (kill $$(cat /tmp/meet-api.pid) 2>/dev/null; rm -f /tmp/meet-api.pid; exit 1)
	@echo "🛑 Stopping application..."
	@kill $$(cat /tmp/meet-api.pid) 2>/dev/null || true
	@rm -f /tmp/meet-api.pid
	@echo "✅ E2E tests completed"

.PHONY: test-e2e-docker
test-e2e-docker:
	@echo "🐳 Starting test environment with Docker..."
	@docker-compose -f docker-compose.test.yml up -d
	@echo "⏳ Waiting for services to be ready..."
	@sleep 10
	@echo "🧪 Running E2E tests against Docker..."
	@APP_URL=http://localhost:8081 APP_ENV=test go test -v ./test/e2e/... -count=1 || (docker-compose -f docker-compose.test.yml down; exit 1)
	@echo "🛑 Stopping Docker services..."
	@docker-compose -f docker-compose.test.yml down
	@echo "✅ E2E tests with Docker completed"

.PHONY: test-e2e-docker-clean
test-e2e-docker-clean:
	@echo "🐳 Starting fresh test environment..."
	@docker-compose -f docker-compose.test.yml down -v
	@docker-compose -f docker-compose.test.yml up -d
	@echo "⏳ Waiting for services to be ready..."
	@sleep 10
	@echo "🧪 Running E2E tests..."
	@APP_URL=http://localhost:8081 APP_ENV=test go test -v ./test/e2e/... -count=1 || (docker-compose -f docker-compose.test.yml down -v; exit 1)
	@echo "🛑 Cleaning up..."
	@docker-compose -f docker-compose.test.yml down -v
	@echo "✅ E2E tests completed and cleaned up"

.PHONY: test-e2e-docker-up
test-e2e-docker-up:
	@echo "🐳 Starting test environment..."
	@docker-compose -f docker-compose.test.yml up -d
	@echo "✅ Test environment ready at http://localhost:8081"
	@echo "💡 Run tests with: APP_URL=http://localhost:8081 make test-e2e"
	@echo "💡 Stop with: make test-e2e-docker-down"

.PHONY: test-e2e-docker-down
test-e2e-docker-down:
	@echo "🛑 Stopping test environment..."
	@docker-compose -f docker-compose.test.yml down -v
	@echo "✅ Test environment stopped"

.PHONY: watch
watch:
	@~/go/bin/air -c air.conf
	
.PHONY: lint
lint:
	@golangci-lint run --timeout=5m

.PHONY: lint-fix
lint-fix:
	@golangci-lint run --timeout=5m --fix

.PHONY: test-coverage
test-coverage:
	@mkdir -p coverage
	@go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic $(shell go list ./... | grep -v -E "(cmd|proto)")
	@go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated: coverage/coverage.html"

.PHONY: test-coverage-report
test-coverage-report:
	@mkdir -p coverage
	@go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic $(shell go list ./... | grep -v -E "(cmd|proto)")
	@go tool cover -func=coverage/coverage.out

.PHONY: coverage-by-package
coverage-by-package:
	@echo "📊 Coverage by package:"
	@go test -cover $(shell go list ./... | grep -v -E "(cmd|proto)") | grep -E "^(ok|FAIL)" | sed 's|github.com/salahfarzin/meet/||' | sort

.PHONY: cover-all-pkgs
cover-all-pkgs:
	@echo "📊 Generating coverage reports for all packages..."
	@mkdir -p coverage/packages
	@for pkg in $$(go list ./... | grep -v -E "(cmd|main|testutils)" | head -10); do \
		PKG_NAME=$$(echo "$$pkg" | sed 's|github.com/salahfarzin/notification/||; s|/|_|g'); \
		echo "📦 Processing $$PKG_NAME..."; \
		go test -coverprofile=coverage/packages/$${PKG_NAME}.out $$pkg 2>/dev/null || echo "⚠️  No tests for $$PKG_NAME"; \
		if [ -f coverage/packages/$${PKG_NAME}.out ]; then \
			go tool cover -html=coverage/packages/$${PKG_NAME}.out -o coverage/packages/$${PKG_NAME}.html; \
			COVERAGE=$$(go tool cover -func=coverage/packages/$${PKG_NAME}.out | grep total | awk '{print $$3}'); \
			echo "   ✅ $$PKG_NAME: $$COVERAGE"; \
		fi; \
	done
	@echo "📄 Individual package reports: coverage/packages/"
	@echo "📊 Summary:"
	@ls -la coverage/packages/ | grep -E "\.(html|out)$$" | wc -l | xargs echo "   Generated reports for packages"

.PHONY: cover-all-summary
cover-all-summary:
	@echo "📊 Detailed Coverage Summary by Package:"
	@mkdir -p coverage
	@go test -coverprofile=coverage/all.out $(shell go list ./... | grep -v -E "(cmd|proto)") > /dev/null 2>&1 || true
	@echo ""
	@echo "🏆 Overall Coverage:"
	@go tool cover -func=coverage/all.out | tail -1
	@echo ""
	@echo "📦 Per-Package Breakdown:"
	@go tool cover -func=coverage/all.out | grep -v "total:" | sort -k3 -nr | head -10

.PHONY: benchmark
benchmark:
	@go test -bench=. -benchmem ./...

.PHONY: security-scan
security-scan:
	@gosec ./...

.PHONY: quality-check
quality-check: lint test-coverage security-scan
	@echo "✅ All quality checks passed!"

.PHONY: complexity-check
complexity-check:
	@gocyclo -over 10 .

.PHONY: quality-gate
quality-gate:
	@./scripts/quality-gate.sh
	# @docker run -it --rm \
	# 	-w "/go/src/github.com/cosmtrek/hub" \
	# 	-v .:/go/src/github.com/cosmtrek/hub \
	# 	-p 3000:3000 \
    # 	cosmtrek/air
	