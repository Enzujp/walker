.PHONY: build test check demo smoke
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
