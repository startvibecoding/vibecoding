.PHONY: build test lint fmt clean install

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
