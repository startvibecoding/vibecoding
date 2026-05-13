.PHONY: build build-windows build-linux build-darwin build-all test lint fmt clean install deb tarball dist clean-all help

help:
	@echo "Available targets:"
	@echo "  build          - Build the binary for current platform"
	@echo "  build-windows  - Build for Windows (amd64 and arm64)"
	@echo "  build-linux    - Build for Linux (amd64 and arm64)"
	@echo "  build-darwin   - Build for macOS (amd64 and arm64)"
	@echo "  build-all      - Cross-compile for all platforms"
	@echo "  install        - Install via go install"
	@echo "  test           - Run tests"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  clean          - Remove build artifacts"
	@echo "  run            - Build and run"
	@echo "  deb            - Build .deb package"
	@echo "  tarball        - Build .tar.gz package"
	@echo "  dist           - Build all distribution packages"
	@echo "  clean-all      - Clean everything including dist"
	@echo "  help           - Show this help"

BINARY_NAME=vibecoding
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X github.com/fuckvibecoding/vibecoding/internal/ua.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/vibecoding

install:
	go install $(LDFLAGS) ./cmd/vibecoding

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

clean:
	rm -rf bin/

run: build
	./bin/$(BINARY_NAME)

# Platform-specific builds
build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/vibecoding
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-arm64.exe ./cmd/vibecoding

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/vibecoding
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/vibecoding

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/vibecoding
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/vibecoding

# Cross compilation (all platforms)
build-all: build-linux build-darwin build-windows
	@echo "Built all platform binaries in bin/"

# Package builds
deb:
	./scripts/build-deb.sh

tarball:
	./scripts/build-tarball.sh

dist: deb tarball
	@echo "All distribution packages built in dist/"

# Clean everything including dist
clean-all: clean
	rm -rf dist/
