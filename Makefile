GO ?= go

.PHONY: run build test test-race vet fmt fmt-check smoke check install

run:
	$(GO) run ./cmd/modeltui

build:
	$(GO) build -o bin/modeltui ./cmd/modeltui

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w cmd internal

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)

smoke: build
	python3 scripts/smoke.py ./bin/modeltui

check: fmt-check vet test-race smoke

install:
	$(GO) install ./cmd/modeltui
