CC ?= gcc
GO ?= go
GO_BUILD_CACHE ?= $(CURDIR)/.cache/go-build

BIN_DIR = bin
GO_SRC = $(shell find src/ -name "*.go")
NFQWS_DIR = vendor/zapret/nfq
NFQWS_SRC = $(shell find $(NFQWS_DIR) -name "*.c" -o -name "*.h")

TARGET_CLI = $(BIN_DIR)/discord-bypass
TARGET_NFQWS = $(BIN_DIR)/discord-bypass-nfqws

all: build

build: $(TARGET_CLI) $(TARGET_NFQWS)

$(TARGET_CLI): $(GO_SRC)
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(GO_BUILD_CACHE)
	GOCACHE=$(GO_BUILD_CACHE) $(GO) build -v -ldflags="-s -w" -o $(TARGET_CLI) ./src/cmd/discord-bypass

$(TARGET_NFQWS): $(NFQWS_SRC)
	@mkdir -p $(BIN_DIR)
	$(MAKE) -C $(NFQWS_DIR) OUT_PATH="$(abspath $(TARGET_NFQWS))"

test: build
	@echo "Running automated test suites..."
	@mkdir -p $(GO_BUILD_CACHE)
	GOCACHE=$(GO_BUILD_CACHE) $(GO) test -v ./tests/...

install: build
	@echo "Installing discord-bypass..."
	sudo ./install.sh

uninstall:
	@echo "Uninstalling discord-bypass..."
	sudo ./uninstall.sh

clean:
	rm -rf $(BIN_DIR)
	$(MAKE) -C $(NFQWS_DIR) clean
	rm -rf debian/discord-bypass* packaging/build

deb: build
	@echo "Building Debian/Ubuntu .deb package..."
	./packaging/build_deb.sh

.PHONY: all build test install uninstall clean deb
