CC ?= gcc
CFLAGS ?= -O2 -Wall -Wextra -D_GNU_SOURCE
LDFLAGS ?= -lnetfilter_queue -lnfnetlink

SRCS = main.c desync.c
OBJS = $(SRCS:.c=.o)
TARGET = ../../../bin/discord-bypass-nfqws

all: $(TARGET)

$(TARGET): $(OBJS)
	@mkdir -p ../../../bin
	$(CC) $(CFLAGS) -o $@ $(OBJS) $(LDFLAGS)

%.o: %.c desync.h
	$(CC) $(CFLAGS) -c $< -o $@

clean:
	rm -f $(OBJS) $(TARGET)

.PHONY: all clean
