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

build:
	make proto
	@go build -o tmp/meets-api

dev:
	@go run main.go

run: build
	@./bin/meet-api

test:
	@go test -v ./...

watch:
	@~/go/bin/air -c air.conf
	# @docker run -it --rm \
	# 	-w "/go/src/github.com/cosmtrek/hub" \
	# 	-v .:/go/src/github.com/cosmtrek/hub \
	# 	-p 3000:3000 \
    # 	cosmtrek/air

.PHONY: watch build run test meets.pb