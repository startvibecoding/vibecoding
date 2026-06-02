.PHONY: help build build-all install test test-vendored lint fmt clean run
.PHONY: build-linux build-linux-musl build-darwin build-windows
.PHONY: dist dist-linux dist-darwin dist-windows dist-deb dist-tarball dist-zip
.PHONY: clean-all checksums
.PHONY: npm-version npm-binaries npm-packages npm-pack npm-publish-all npm-publish-pre npm-publish
.PHONY: prepare-vendored

# Variables
BINARY_NAME=vibecoding
VERSION=$(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X github.com/startvibecoding/vibecoding/internal/ua.Version=$(VERSION)"
GOBUILD_FLAGS=-trimpath
DIST_DIR=dist
CHECKSUM_FILE=$(DIST_DIR)/checksums.txt

# UPX compression (skip for macOS - not supported)
USE_UPX ?= true
ifeq ($(shell which upx 2>/dev/null),)
USE_UPX = false
endif
ifeq ($(USE_UPX),true)
UPX_CMD = upx -9
else
UPX_CMD = @true
endif

# Platforms and architectures (for reference)
# linux: amd64 arm64
# darwin: amd64 arm64
# windows: amd64 arm64

# Default target
help:
	@echo "VibeCoding Build System"
	@echo ""
	@echo "Build targets:"
	@echo "  build            Build for current platform"
	@echo "  build-linux      Build for Linux (amd64, arm64)"
	@echo "  build-linux-musl Build for Linux musl (amd64)"
	@echo "  build-darwin     Build for macOS (amd64, arm64)"
	@echo "  build-windows    Build for Windows (amd64, arm64)"
	@echo "  build-all        Build for all platforms and architectures"
	@echo "  prepare-vendored Extract rg/fd binaries for go:embed"
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
	@echo "  npm-version       Sync version to npm package"
	@echo "  npm-packages      Build platform-specific npm packages"
	@echo "  npm-pack          Pack main + all platform packages"
	@echo "  npm-publish-all   Publish main + all platform packages"
	@echo "  npm-publish-pre   Publish pre-release packages"
	@echo "  npm-binaries      [Legacy] Build all binaries into single package"
	@echo "  npm-publish       [Legacy] Publish main package only"
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

# Prepare vendored binaries for go:embed
prepare-vendored:
	./scripts/prepare-vendored.sh

# Build for current platform (requires prepare-vendored first)
build: prepare-vendored
	go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/vibecoding

# Platform builds
build-linux: prepare-vendored
	@echo "Building for Linux..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/vibecoding
	GOOS=linux GOARCH=arm64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/vibecoding
	@echo "Compressing Linux amd64 binary with UPX..."
	$(UPX_CMD) bin/$(BINARY_NAME)-linux-amd64

# musl: static build with CGO_ENABLED=0, arm64 not commonly needed
build-linux-musl: prepare-vendored
	@echo "Building for Linux musl..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-musl-amd64 ./cmd/vibecoding
	@echo "Compressing Linux musl binary with UPX..."
	$(UPX_CMD) bin/$(BINARY_NAME)-linux-musl-amd64

build-darwin: prepare-vendored
	@echo "Building for macOS..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/vibecoding
	GOOS=darwin GOARCH=arm64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/vibecoding

build-windows: prepare-vendored
	@echo "Building for Windows..."
	@mkdir -p bin
	GOOS=windows GOARCH=amd64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/vibecoding
	GOOS=windows GOARCH=arm64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-arm64.exe ./cmd/vibecoding
	@echo "Compressing Windows amd64 binary with UPX..."
	$(UPX_CMD) bin/$(BINARY_NAME)-windows-amd64.exe

# Build all platforms
build-all: prepare-vendored build-linux build-linux-musl build-darwin build-windows
	@echo ""
	@echo "Build complete! Binaries in bin/"
	@ls -lh bin/

# Install
install:
	go install $(GOBUILD_FLAGS) $(LDFLAGS) ./cmd/vibecoding

# Test
test: prepare-vendored test-vendored
	go test -v -race ./...

test-vendored:
	@case "$$(go env GOOS)-$$(go env GOARCH)" in \
		windows-*) ext=".exe" ;; \
		*) ext="" ;; \
	esac; \
	dir="internal/vendored/bin/$$(go env GOOS)-$$(go env GOARCH)"; \
	if [ ! -f "$$dir/rg$$ext" ] || [ ! -f "$$dir/fd$$ext" ]; then \
		echo "Missing vendored rg/fd for $$(go env GOOS)-$$(go env GOARCH)."; \
		echo "Run: make prepare-vendored"; \
		exit 1; \
	fi

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
	rm -f npm/*.tgz

# Run
run: build
	./bin/$(BINARY_NAME)

# Distribution: tar.gz for Linux and macOS
dist-tarball: build-linux build-linux-musl build-darwin
	@echo ""
	@echo "Creating tarball packages..."
	@for os in linux darwin; do \
		for arch in amd64 arm64; do \
			echo "  Packaging $(BINARY_NAME)-$${os}-$${arch}.tar.gz..."; \
			./scripts/build-tarball.sh $${os} $${arch} $(VERSION); \
		done; \
	done
	@echo "  Packaging $(BINARY_NAME)-linux-musl-amd64.tar.gz..."; \
	./scripts/build-tarball.sh linux-musl amd64 $(VERSION)

# Distribution: deb for Linux
dist-deb: build-linux build-linux-musl
	@echo ""
	@echo "Creating Debian packages..."
	@for arch in amd64 arm64; do \
		echo "  Packaging $(BINARY_NAME)_$(VERSION)_$${arch}.deb..."; \
		./scripts/build-deb.sh $${arch} $(VERSION); \
	done
	@echo "  Packaging $(BINARY_NAME)_$(VERSION)_amd64-musl.deb..."; \
	./scripts/build-deb.sh amd64-musl $(VERSION)

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

# Legacy: build all binaries into single package (use npm-packages instead)
npm-binaries: build-all
	@echo "WARNING: npm-binaries is deprecated, use npm-packages instead" >&2
	./scripts/build-npm.sh

# Build platform-specific packages
npm-packages: build-all
	./scripts/build-npm-packages.sh

# Pack main + platform packages
npm-pack: npm-version npm-packages
	@echo "Packing platform packages..."
	@for d in npm/packages/*/; do \
		if [ -f "$$d/package.json" ]; then \
			echo "  Packing $$(basename $$d)..."; \
			cd "$$d" && npm pack && cd - > /dev/null; \
			mv "$$d"/*.tgz npm/ 2>/dev/null || true; \
		fi; \
	done
	@echo "Packing main package..."
	cd npm && npm pack
	@echo "Done. Tarballs in npm/"

# Publish platform packages first, then main
npm-publish-all: npm-version npm-packages
	@echo "Publishing platform packages..."
	@for d in npm/packages/*/; do \
		if [ -f "$$d/package.json" ]; then \
			echo "  Publishing $$(basename $$d)..."; \
			cd "$$d" && npm publish --tag latest && cd - > /dev/null; \
		fi; \
	done
	@echo "Publishing main package..."
	cd npm && npm publish --tag latest
	@echo "Published all packages!"

# Publish pre-release
npm-publish-pre: npm-version npm-packages
	@echo "Publishing platform packages (pre-release)..."
	@for d in npm/packages/*/; do \
		if [ -f "$$d/package.json" ]; then \
			echo "  Publishing $$(basename $$d)..."; \
			cd "$$d" && npm publish --tag next && cd - > /dev/null; \
		fi; \
	done
	@echo "Publishing main package (pre-release)..."
	cd npm && npm publish --tag next
	@echo "Published all packages (pre-release)!"

# Legacy: publish main package only (use npm-publish-all instead)
npm-publish: npm-version npm-binaries
	@echo "WARNING: npm-publish is deprecated, use npm-publish-all instead" >&2
	cd npm && npm publish --tag latest
