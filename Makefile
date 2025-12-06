BINARY    := gti
BUILD_DIR := bin
THRESHOLD := 90
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test test-integration lint fmt vet fix coverage install clean snapshot check-release

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/gti

test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@COV=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | tr -d '%'); \
	 [ $$(echo "$$COV < $(THRESHOLD)" | bc) -eq 1 ] \
	   && { echo "FAIL: $$COV% < $(THRESHOLD)%"; exit 1; } \
	   || echo "OK: $$COV%"

test-integration:
	go test -race -tags integration ./tests/integration/...

fix:    ; go fix ./...
lint:   ; golangci-lint run ./...
fmt:    ; gofmt -w . && goimports -w .
vet:    ; go vet ./...
coverage: ; go tool cover -html=coverage.out -o coverage.html

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)

snapshot:
	goreleaser release --snapshot --clean

check-release:
	goreleaser check

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html dist/
