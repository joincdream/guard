.PHONY: build install test

BINARY=guard
INSTALL_DIR=$(HOME)/.local/bin

build:
	go build -o $(BINARY) ./cmd/guard

install:
	mkdir -p $(INSTALL_DIR)
	go build -o $(INSTALL_DIR)/$(BINARY) ./cmd/guard
	@echo "✅ Installed $(BINARY) to $(INSTALL_DIR)/$(BINARY)"

test:
	go test -v ./...
