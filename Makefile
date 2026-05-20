.PHONY: help build build-all install test lint fmt clean run
.PHONY: build-linux build-linux-musl build-darwin build-windows
.PHONY: dist dist-linux dist-darwin dist-windows dist-deb dist-tarball dist-zip
.PHONY: clean-all checksums
.PHONY: npm-version npm-publish npm-publish-all npm-pack npm-pack-all
.PHONY: prepare-vendored

# Variables
BINARY_NAME=vibecoding
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X github.com/startvibecoding/vibecoding/internal/ua.Version=$(VERSION)"
DIST_DIR=dist
CHECKSUM_FILE=$(DIST_DIR)/checksums.txt
VENDORED_BIN_DIR=bin/vendored

# Platforms and architectures
PLATFORMS=linux darwin windows
ARCHS=amd64 arm64

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
	@echo "  npm-pack          Pack main + all platform packages"
	@echo "  npm-publish-all   Publish main + all platform packages"
	@echo "  npm-publish       Publish main package only (legacy)"
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
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/vibecoding

vendored-tools:
	@echo "Extracting vendored fd/rg for all supported targets..."
	@mkdir -p $(VENDORED_BIN_DIR)
	@for target in \
		linux amd64 \
		linux arm64 \
		linux-musl amd64 \
		darwin amd64 \
		darwin arm64 \
		windows amd64 \
		windows arm64; do \
		set -- $$target; \
		os=$$1; arch=$$2; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		./scripts/extract-vendored-tool.sh fd $$os $$arch $(VENDORED_BIN_DIR)/fd-$$os-$$arch$$ext; \
		./scripts/extract-vendored-tool.sh rg $$os $$arch $(VENDORED_BIN_DIR)/rg-$$os-$$arch$$ext; \
	done

# Platform builds
build-linux:
	@echo "Building for Linux..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/vibecoding
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/vibecoding

build-linux-musl:
	@echo "Building for Linux musl..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-musl-amd64 ./cmd/vibecoding

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
build-all: build-linux build-linux-musl build-darwin build-windows vendored-tools
	@echo ""
	@echo "Build complete! Binaries in bin/"
	@ls -lh bin/
	@echo ""
	@echo "Vendored tool binaries in $(VENDORED_BIN_DIR)/"
	@ls -lh $(VENDORED_BIN_DIR)/

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
dist-tarball: build-linux build-linux-musl build-darwin vendored-tools
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
dist-deb: build-linux build-linux-musl vendored-tools
	@echo ""
	@echo "Creating Debian packages..."
	@for arch in amd64 arm64; do \
		echo "  Packaging $(BINARY_NAME)_$(VERSION)_$${arch}.deb..."; \
		./scripts/build-deb.sh $${arch} $(VERSION); \
	done
	@echo "  Packaging $(BINARY_NAME)_$(VERSION)_amd64-musl.deb..."; \
	./scripts/build-deb.sh amd64-musl $(VERSION)

# Distribution: zip for Windows
dist-zip: build-windows vendored-tools
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

# Legacy: build all binaries into single package
npm-binaries: build-all
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

# Legacy: publish main package only
npm-publish: npm-version npm-binaries
	cd npm && npm publish --tag latest
