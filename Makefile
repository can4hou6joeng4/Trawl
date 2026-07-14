APP_NAME := legal-extractor
BUILD_DIR := build/bin

# Default target: Build release-grade desktop artifacts
all: mac-universal windows-installer

# ===== Code Quality =====

# Run all tests with race detection
test:
	@echo "🧪 Running tests..."
	go test -race -count=1 ./...

# Run golangci-lint
lint:
	@echo "🔍 Running linter..."
	golangci-lint run ./...

# Format all Go source files
fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...

# Run go vet
vet:
	@echo "🔬 Running go vet..."
	go vet ./...

# All quality checks
check: fmt vet lint test
	@echo "✅ All checks passed."

# ===== Build Targets =====

# Build for macOS (Universal: amd64 + arm64)
mac: mac-universal

mac-universal:
	@echo "🍎 Building for macOS (Universal Binary)..."
	wails build -platform darwin/universal

mac-amd64:
	@echo "🍎 Building for macOS (Intel)..."
	wails build -platform darwin/amd64

mac-arm64:
	@echo "🍎 Building for macOS (Apple Silicon)..."
	wails build -platform darwin/arm64

# Build for Windows
# Requires MinGW-w64 to be installed: brew install mingw-w64
windows:
	@echo "🪟 Building for Windows (amd64)..."
	wails build -platform windows/amd64

windows-installer:
	@echo "🪟 Building Windows installer (amd64)..."
	wails build -platform windows/amd64 -nsis

# OCR 中继网关（自部署，见 docs/ocr-gateway-deploy.md）
gateway:
	@echo "🛰  Building OCR gateway..."
	go build -trimpath -ldflags "-s -w" -o $(BUILD_DIR)/ocr-gateway ./cmd/ocr-gateway

# 为客户机器生成激活码（所有者运维）：make license MACHINE=XXXXXXXX
# 密钥来源：环境变量 LICENSE_SECRET 或 ~/.secrets/license-secret
license:
	@test -n "$(MACHINE)" || { echo "用法: make license MACHINE=机器码（应用激活界面显示的 8 位码）"; exit 1; }
	@go run ./cmd/license-gen $(MACHINE)

# ===== Setup =====

# Install dependencies (Homebrew required)
deps:
	@echo "🛠 Checking dependencies..."
	@if ! command -v brew >/dev/null 2>&1; then \
		echo "❌ Homebrew not found. Please install Homebrew first."; \
		exit 1; \
	fi
	@echo "📦 Installing mingw-w64 for Windows cross-compilation..."
	brew install mingw-w64
	@echo "📦 Installing golangci-lint..."
	brew install golangci-lint
	@echo "✅ Done."

# Clean build directory
clean:
	@echo "🧹 Cleaning build directory..."
	rm -rf $(BUILD_DIR)/*
	rm -rf dist/
	@echo "✅ Done."

.PHONY: all test lint fmt vet check mac mac-universal mac-amd64 mac-arm64 windows windows-installer gateway license deps clean
