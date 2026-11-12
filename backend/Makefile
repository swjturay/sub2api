.PHONY: build generate test test-unit test-integration test-e2e

VERSION ?= $(shell ./scripts/resolve-version.sh)
LDFLAGS ?= -s -w -X main.Version=$(VERSION)

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -trimpath -o bin/server ./cmd/server

generate:
	go generate ./ent
	go generate ./cmd/server

test:
	go test ./...
	golangci-lint run ./...

# Heap growth must be measured in a fresh process, as in the CCE release gate.
# Keep the complete inventory and the original bound; isolate only this test.
test-unit:
	go test -tags=unit -skip '^TestInflightEstimate_AccountMappingNoDBAndBoundedMemory$$' ./...
	go test -tags=unit ./internal/service -run '^TestInflightEstimate_AccountMappingNoDBAndBoundedMemory$$' -count=1

test-integration:
	go test -tags=integration ./...

test-e2e:
	./scripts/e2e-test.sh

test-e2e-local:
	go test -tags=e2e -v -timeout=300s ./internal/integration/...
