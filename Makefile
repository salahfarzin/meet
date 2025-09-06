build:
	@go build -o tmp/analyse-api

dev:
	@go run main.go
run: build
	@./bin/analyse-api

test:
	@go test -v ./...

watch:
	@~/go/bin/air air.conf
	# @docker run -it --rm \
	# 	-w "/go/src/github.com/cosmtrek/hub" \
	# 	-v .:/go/src/github.com/cosmtrek/hub \
	# 	-p 3000:3000 \
    # 	cosmtrek/air

.PHONY: watch build run test