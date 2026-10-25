BINARY_NAME ?= envlock
ALIAS_NAME ?= el
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "0.2.1")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")
BUILD_DIR ?= bin
DIST_DIR ?= dist

LDFLAGS := -s -w \
	-X github.com/itskdhere/envlock/cmd.Version=$(VERSION) \
	-X github.com/itskdhere/envlock/cmd.Commit=$(COMMIT) \
	-X github.com/itskdhere/envlock/cmd.Date=$(DATE)

.PHONY: all build test release clean

all: build

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(ALIAS_NAME) 2>/dev/null || copy $(BUILD_DIR)\$(BINARY_NAME).exe $(BUILD_DIR)\$(ALIAS_NAME).exe 2>nul || true

test:
	go test -v ./...

release: clean
	@mkdir -p $(DIST_DIR)
	# Linux AMD64
	@mkdir -p $(DIST_DIR)/tmp_linux_amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/tmp_linux_amd64/$(BINARY_NAME) main.go
	cp $(DIST_DIR)/tmp_linux_amd64/$(BINARY_NAME) $(DIST_DIR)/tmp_linux_amd64/$(ALIAS_NAME)
	cp README.md $(DIST_DIR)/tmp_linux_amd64/
	cp $(DIST_DIR)/tmp_linux_amd64/$(BINARY_NAME) $(DIST_DIR)/$(BINARY_NAME)-linux-amd64
	tar -czf $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_linux_amd64.tar.gz -C $(DIST_DIR)/tmp_linux_amd64 .
	rm -rf $(DIST_DIR)/tmp_linux_amd64

	# Linux ARM64
	@mkdir -p $(DIST_DIR)/tmp_linux_arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/tmp_linux_arm64/$(BINARY_NAME) main.go
	cp $(DIST_DIR)/tmp_linux_arm64/$(BINARY_NAME) $(DIST_DIR)/tmp_linux_arm64/$(ALIAS_NAME)
	cp README.md $(DIST_DIR)/tmp_linux_arm64/
	cp $(DIST_DIR)/tmp_linux_arm64/$(BINARY_NAME) $(DIST_DIR)/$(BINARY_NAME)-linux-arm64
	tar -czf $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_linux_arm64.tar.gz -C $(DIST_DIR)/tmp_linux_arm64 .
	rm -rf $(DIST_DIR)/tmp_linux_arm64

	# macOS Apple Silicon (ARM64)
	@mkdir -p $(DIST_DIR)/tmp_darwin_arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/tmp_darwin_arm64/$(BINARY_NAME) main.go
	cp $(DIST_DIR)/tmp_darwin_arm64/$(BINARY_NAME) $(DIST_DIR)/tmp_darwin_arm64/$(ALIAS_NAME)
	cp README.md $(DIST_DIR)/tmp_darwin_arm64/
	cp $(DIST_DIR)/tmp_darwin_arm64/$(BINARY_NAME) $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64
	tar -czf $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_darwin_arm64.tar.gz -C $(DIST_DIR)/tmp_darwin_arm64 .
	rm -rf $(DIST_DIR)/tmp_darwin_arm64

	# macOS Intel (AMD64)
	@mkdir -p $(DIST_DIR)/tmp_darwin_amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/tmp_darwin_amd64/$(BINARY_NAME) main.go
	cp $(DIST_DIR)/tmp_darwin_amd64/$(BINARY_NAME) $(DIST_DIR)/tmp_darwin_amd64/$(ALIAS_NAME)
	cp README.md $(DIST_DIR)/tmp_darwin_amd64/
	cp $(DIST_DIR)/tmp_darwin_amd64/$(BINARY_NAME) $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64
	tar -czf $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_darwin_amd64.tar.gz -C $(DIST_DIR)/tmp_darwin_amd64 .
	rm -rf $(DIST_DIR)/tmp_darwin_amd64

	# Windows AMD64
	@mkdir -p $(DIST_DIR)/tmp_windows_amd64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/tmp_windows_amd64/$(BINARY_NAME).exe main.go
	cp $(DIST_DIR)/tmp_windows_amd64/$(BINARY_NAME).exe $(DIST_DIR)/tmp_windows_amd64/$(ALIAS_NAME).exe
	cp README.md $(DIST_DIR)/tmp_windows_amd64/
	cp $(DIST_DIR)/tmp_windows_amd64/$(BINARY_NAME).exe $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe
	(cd $(DIST_DIR)/tmp_windows_amd64 && zip -r ../$(BINARY_NAME)_$(VERSION)_windows_amd64.zip .)
	rm -rf $(DIST_DIR)/tmp_windows_amd64

	# Generate SHA-256 Checksums
	(cd $(DIST_DIR) && rm -f checksums.txt && (sha256sum $(BINARY_NAME)* 2>/dev/null || shasum -a 256 $(BINARY_NAME)*) > checksums.txt)

clean:
	@rm -rf $(BUILD_DIR) $(DIST_DIR) 2>/dev/null || true
