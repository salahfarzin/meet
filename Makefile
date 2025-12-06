PROTO_DIR := proto
PROTO_SRC := $(wildcard $(PROTO_DIR)/*.proto)
GO_OUT := .
GOOGLEAPIS_DIR := $(HOME)/go/src/googleapis

.PHONY: generate-proto
generate-proto:
	protoc \
		-I. -I$(GOOGLEAPIS_DIR) \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GO_OUT) \
		--go-grpc_out=$(GO_OUT) \
		--grpc-gateway_out=$(GO_OUT) \
		$(PROTO_SRC)

.PHONY: migrate
migrate:
	@if [ ! -f .env ]; then echo "❌ .env file not found"; exit 1; fi
	@set -a; source .env; set +a; migrate -path migrations -database "mysql://notification_user:root@tcp(127.0.0.1:3306)/notification_db" up

.PHONY: migrate-down
migrate-down:
	@if [ ! -f .env ]; then echo "❌ .env file not found"; exit 1; fi
	@set -a; source .env; set +a; migrate -path migrations -database "mysql://notification_user:root@tcp(127.0.0.1:3306)/notification_db" down

.PHONY: migrate-create
migrate-create:
	@if [ -z "$(name)" ]; then echo "❌ Usage: make migrate-create name=migration_name"; exit 1; fi
	@migrate create -ext sql -dir migrations -seq $(name)

.PHONY: build
build:
	make proto
	@go build -o tmp/meets-api

.PHONY: dev
dev:
	@go run main.go

.PHONY: run
run: build
	@./bin/meet-api

.PHONY: test
test:
	@go test -v ./...

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
	