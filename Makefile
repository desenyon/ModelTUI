.PHONY: run build test install

run:
	go run ./cmd/modeltui

build:
	go build -o bin/modeltui ./cmd/modeltui

test:
	go test ./...

install:
	go install ./cmd/modeltui
