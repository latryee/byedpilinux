# Linux-only vendored build for bol-van/zapret nfqws v72.13.
# Source files are unmodified upstream files; this Makefile is local glue.
CC ?= cc
OUT_PATH ?= nfqws
CPPFLAGS ?=
CFLAGS ?= -O2 -g0
CFLAGS += -std=gnu99 -Wall -Wextra -Wformat=2 -Wformat-security -Wshadow -Wpointer-arith -Wstrict-prototypes -ffunction-sections -fdata-sections
LDFLAGS += -Wl,--gc-sections
LDLIBS += -lz -lnetfilter_queue -lnfnetlink -lmnl
SRC := $(wildcard *.c) $(wildcard crypto/*.c)

.DEFAULT_GOAL := all
all: $(OUT_PATH)

check-deps:
	@command -v pkg-config >/dev/null || { echo "error: pkg-config is required (install pkg-config, libnetfilter-queue-dev, libmnl-dev, zlib1g-dev)"; exit 1; }
	@pkg-config --exists libnetfilter_queue libmnl zlib || { echo "error: missing nfqws build dependencies (install libnetfilter-queue-dev libmnl-dev zlib1g-dev)"; exit 1; }

$(OUT_PATH): check-deps $(SRC)
	@mkdir -p $(dir $@)
	$(CC) $(CPPFLAGS) $(CFLAGS) -o $@ $(SRC) $(LDFLAGS) $(LDLIBS)

clean:
	rm -f nfqws

.PHONY: all clean check-deps
