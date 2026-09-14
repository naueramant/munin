BINARY  := munin
PREFIX  ?= /usr/local
BINDIR  := $(PREFIX)/bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/naueramant/munin/internal/updater.CurrentVersion=$(VERSION)

.PHONY: build release lint test install uninstall

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

lint:
	golangci-lint run ./...

test:
	go test -race ./...

install: build
	install -Dm755 $(BINARY) $(BINDIR)/$(BINARY)

uninstall:
	rm -f $(BINDIR)/$(BINARY)
