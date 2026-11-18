.PHONY: build install test

BINARY=guard
INSTALL_DIR=$(HOME)/.local/bin

build:
	go build -o $(BINARY) .

install:
	mkdir -p $(INSTALL_DIR)
	go build -o $(INSTALL_DIR)/$(BINARY) .
	@echo "✅ Installed $(BINARY) to $(INSTALL_DIR)/$(BINARY)"

test:
	go test -v ./...
