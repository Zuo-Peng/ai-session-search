VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
PREFIX ?= $(HOME)/.local

.PHONY: build clean install uninstall

build:
	go build $(LDFLAGS) -o ais ./cmd/ais

install: build
	mkdir -p $(PREFIX)/bin
	cp ais $(PREFIX)/bin/ais

uninstall:
	rm -f $(PREFIX)/bin/ais

clean:
	rm -f ais
