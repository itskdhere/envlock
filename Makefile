BINARY_NAME=envlock
ALIAS_NAME=el
VERSION=0.1.0
BUILD_DIR=bin
DIST_DIR=dist

LDFLAGS=-ldflags="-s -w -X github.com/itskdhere/envlock/cmd.Version=$(VERSION)"

.PHONY: all build release test clean

all: build

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(ALIAS_NAME) 2>/dev/null || copy $(BUILD_DIR)\$(BINARY_NAME).exe $(BUILD_DIR)\$(ALIAS_NAME).exe 2>nul || true

test:
	go test -v ./...

release: clean
	# Linux AMD64 & ARM64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 main.go
	# macOS Apple Silicon & Intel
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 main.go
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	# Windows AMD64
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR)

