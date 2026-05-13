.PHONY: build test lint fmt clean install deb tarball dist clean-all help

help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  install     - Install via go install"
	@echo "  test        - Run tests"
	@echo "  lint        - Run linter"
	@echo "  fmt         - Format code"
	@echo "  clean       - Remove build artifacts"
	@echo "  run         - Build and run"
	@echo "  build-all   - Cross-compile for all platforms"
	@echo "  deb         - Build .deb package"
	@echo "  tarball     - Build .tar.gz package"
	@echo "  dist        - Build all distribution packages"
	@echo "  clean-all   - Clean everything including dist"
	@echo "  help        - Show this help"

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

# Cross compilation
build-all:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/vibecoding
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/vibecoding
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/vibecoding
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/vibecoding

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
