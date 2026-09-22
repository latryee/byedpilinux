CC ?= gcc
GO ?= go

BIN_DIR = bin
GO_SRC = $(shell find src/ -name "*.go")
NFQWS_SRC = $(shell find src/backend/nfqws/ -name "*.c" -o -name "*.h")

TARGET_CLI = $(BIN_DIR)/discord-bypass
TARGET_NFQWS = $(BIN_DIR)/discord-bypass-nfqws

all: build

build: $(TARGET_CLI) $(TARGET_NFQWS)

$(TARGET_CLI): $(GO_SRC)
	@mkdir -p $(BIN_DIR)
	$(GO) build -v -ldflags="-s -w" -o $(TARGET_CLI) ./src/cmd/discord-bypass

$(TARGET_NFQWS): $(NFQWS_SRC)
	@mkdir -p $(BIN_DIR)
	$(MAKE) -C src/backend/nfqws

test: build
	@echo "Running automated test suites..."
	$(GO) test -v ./tests/...

install: build
	@echo "Installing discord-bypass..."
	sudo ./install.sh

uninstall:
	@echo "Uninstalling discord-bypass..."
	sudo ./uninstall.sh

clean:
	rm -rf $(BIN_DIR)
	$(MAKE) -C src/backend/nfqws clean
	rm -rf debian/discord-bypass* packaging/build

deb: build
	@echo "Building Debian/Ubuntu .deb package..."
	./packaging/build_deb.sh

.PHONY: all build test install uninstall clean deb
