.PHONY: build install clean test lint fmt vet all help

# Variables
BINARY=androideai
VERSION=1.0.0
GIT_COMMIT=$(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X main.v=${VERSION} -X main.commit=${GIT_COMMIT} -X main.buildTime=${BUILD_TIME} -X github.com/pablofelipe1207/androideia/internal/version.Version=${VERSION} -X github.com/pablofelipe1207/androideia/internal/version.GitCommit=${GIT_COMMIT} -X github.com/pablofelipe1207/androideia/internal/version.BuildDate=${BUILD_TIME}"
INSTALL_DIR=$(HOME)/.local/bin

# Default target
all: build

# Build the binary (without tree-sitter, pure Go)
build:
	CGO_ENABLED=0 go build -tags no_treesitter $(LDFLAGS) -o $(BINARY) .

# Build with tree-sitter (requires CGO)
build-full:
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BINARY) .

# Install to ~/.local/bin
install: build
	@mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)"
	@echo "Add to PATH if needed: export PATH=\"$(INSTALL_DIR):\$$PATH\""

# Uninstall
uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Uninstalled from $(INSTALL_DIR)/$(BINARY)"

# Run tests
test:
	go test ./...

# Run tests with coverage
test-coverage:
	go test -cover ./...

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	gofmt -s -w .

# Run go vet
vet:
	go vet ./...

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/

# Build for all platforms
build-all:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)_linux_amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)_linux_arm64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)_darwin_amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)_darwin_arm64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)_windows_amd64.exe .
	@echo "Built binaries in dist/"

# Cross-compile check
cross-compile:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /dev/null . && echo "Windows AMD64: OK"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /dev/null . && echo "Linux AMD64: OK"
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o /dev/null . && echo "macOS AMD64: OK"
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /dev/null . && echo "macOS ARM64: OK"

# Initialize project in current directory
init:
	./$(BINARY) init

# Index project
index:
	./$(BINARY) index build

# Show help
help:
	@echo "androideai-core build system"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build without tree-sitter (pure Go, no CGO)"
	@echo "  build-full     Build with tree-sitter (requires CGO)"
	@echo "  install        Build and install to $(INSTALL_DIR)"
	@echo "  uninstall      Remove from $(INSTALL_DIR)"
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage"
	@echo "  lint           Run linter"
	@echo "  fmt            Format code"
	@echo "  vet            Run go vet"
	@echo "  clean          Clean build artifacts"
	@echo "  build-all      Build for all platforms"
	@echo "  cross-compile  Check cross-compilation"
	@echo "  init           Initialize project in current directory"
	@echo "  index          Build index for current project"
	@echo "  help           Show this help"
