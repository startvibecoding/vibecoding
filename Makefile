.PHONY: help build build-all install test lint fmt clean run
.PHONY: build-linux build-darwin build-windows
.PHONY: dist dist-linux dist-darwin dist-windows dist-deb dist-tarball dist-zip
.PHONY: clean-all checksums
.PHONY: npm-version npm-publish npm-pack

# Variables
BINARY_NAME=vibecoding
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X github.com/startvibecoding/vibecoding/internal/ua.Version=$(VERSION)"
DIST_DIR=dist
CHECKSUM_FILE=$(DIST_DIR)/checksums.txt

# Platforms and architectures
PLATFORMS=linux darwin windows
ARCHS=amd64 arm64

# Default target
help:
	@echo "VibeCoding Build System"
	@echo ""
	@echo "Build targets:"
	@echo "  build          Build for current platform"
	@echo "  build-linux    Build for Linux (amd64, arm64)"
	@echo "  build-darwin   Build for macOS (amd64, arm64)"
	@echo "  build-windows  Build for Windows (amd64, arm64)"
	@echo "  build-all      Build for all platforms and architectures"
	@echo ""
	@echo "Distribution targets:"
	@echo "  dist           Build all distribution packages"
	@echo "  dist-linux     Build Linux packages (tar.gz + deb)"
	@echo "  dist-darwin    Build macOS packages (tar.gz)"
	@echo "  dist-windows   Build Windows packages (zip)"
	@echo "  dist-deb       Build Debian packages only"
	@echo "  dist-tarball   Build tarball packages only"
	@echo "  dist-zip       Build zip packages only"
	@echo ""
	@echo "NPM targets:"
	@echo "  npm-version    Sync version to npm package"
	@echo "  npm-publish    Publish to npm registry"
	@echo "  npm-pack       Create npm tarball"
	@echo ""
	@echo "Other targets:"
	@echo "  install        Install via go install"
	@echo "  test           Run tests"
	@echo "  lint           Run linter"
	@echo "  fmt            Format code"
	@echo "  clean          Remove build artifacts"
	@echo "  clean-all      Remove everything including dist"
	@echo "  checksums      Generate checksums for all dist files"
	@echo "  run            Build and run"
	@echo "  help           Show this help"

# Build for current platform
build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/vibecoding

# Platform builds
build-linux:
	@echo "Building for Linux..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/vibecoding
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/vibecoding

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/vibecoding
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/vibecoding

build-windows:
	@echo "Building for Windows..."
	@mkdir -p bin
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/vibecoding
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-arm64.exe ./cmd/vibecoding

# Build all platforms
build-all: build-linux build-darwin build-windows
	@echo ""
	@echo "Build complete! Binaries in bin/"
	@ls -lh bin/

# Install
install:
	go install $(LDFLAGS) ./cmd/vibecoding

# Test
test:
	go test -v -race ./...

# Lint
lint:
	golangci-lint run ./...

# Format
fmt:
	gofmt -w .
	goimports -w .

# Clean
clean:
	rm -rf bin/

# Clean all
clean-all: clean
	rm -rf $(DIST_DIR)

# Run
run: build
	./bin/$(BINARY_NAME)

# Distribution: tar.gz for Linux and macOS
dist-tarball: build-linux build-darwin
	@echo ""
	@echo "Creating tarball packages..."
	@for os in linux darwin; do \
		for arch in amd64 arm64; do \
			echo "  Packaging $(BINARY_NAME)-$${os}-$${arch}.tar.gz..."; \
			./scripts/build-tarball.sh $${os} $${arch} $(VERSION); \
		done; \
	done

# Distribution: deb for Linux
dist-deb: build-linux
	@echo ""
	@echo "Creating Debian packages..."
	@for arch in amd64 arm64; do \
		echo "  Packaging $(BINARY_NAME)_$(VERSION)_$${arch}.deb..."; \
		./scripts/build-deb.sh $${arch} $(VERSION); \
	done

# Distribution: zip for Windows
dist-zip: build-windows
	@echo ""
	@echo "Creating Windows zip packages..."
	@for arch in amd64 arm64; do \
		echo "  Packaging $(BINARY_NAME)-windows-$${arch}.zip..."; \
		./scripts/build-zip.sh $${arch} $(VERSION); \
	done

# Platform distributions
dist-linux: dist-deb dist-tarball
	@echo "Linux packages complete!"

dist-darwin: dist-tarball
	@echo "macOS packages complete!"

dist-windows: dist-zip
	@echo "Windows packages complete!"

# Generate checksums
checksums:
	@echo "Generating checksums..."
	@cd $(DIST_DIR) && \
	find . -type f \( -name "*.tar.gz" -o -name "*.deb" -o -name "*.zip" \) | sort | \
	while read f; do \
		sha256sum "$$f"; \
	done > checksums.txt
	@echo "Checksums written to $(CHECKSUM_FILE)"
	@cat $(CHECKSUM_FILE)

# Build all distribution packages
dist: dist-linux dist-darwin dist-windows checksums
	@echo ""
	@echo "=========================================="
	@echo "All distribution packages built!"
	@echo ""
	@echo "Location: $(DIST_DIR)/"
	@echo ""
	@ls -lh $(DIST_DIR)/*/* 2>/dev/null || true
	@echo ""
	@echo "Checksums: $(CHECKSUM_FILE)"
	@echo "=========================================="

# NPM targets
npm-version:
	./scripts/sync-npm-version.sh $(VERSION)

npm-binaries: build-all
	./scripts/build-npm.sh

npm-pack: npm-version npm-binaries
	cd npm && npm pack

npm-publish: npm-version npm-binaries
	cd npm && npm publish --tag latest

npm-publish-pre: npm-version npm-binaries
	cd npm && npm publish --tag next
