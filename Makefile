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
	# @docker run -it --rm \
	# 	-w "/go/src/github.com/cosmtrek/hub" \
	# 	-v .:/go/src/github.com/cosmtrek/hub \
	# 	-p 3000:3000 \
    # 	cosmtrek/air
	