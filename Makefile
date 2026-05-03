PREFIX ?= $(HOME)/.local
BINDIR ?= $(if $(GOBIN),$(GOBIN),$(PREFIX)/bin)
BIN := $(BINDIR)/sessions

.PHONY: build install test

build:
	go build ./cmd/sessions

install:
	mkdir -p "$(BINDIR)"
	go build -o "$(BIN)" ./cmd/sessions

test:
	go test ./...
