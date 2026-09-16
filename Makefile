.PHONY: build test check demo smoke live
build:
	go build -o bin/walker .
test:
	go test -race -cover ./...
check:
	@test -z "$$(gofmt -l .)"
	go vet ./...
	go test -race ./...
demo:
	go run . extract --demo --config example/postman-options.json
smoke: build
	python3 scripts/smoke_test.py
live:
	NODE_PATH=$${NODE_PATH:-/tmp/walker-compat/node_modules} go test -tags integration -race -v ./integration
