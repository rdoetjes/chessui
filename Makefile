.PHONY: all build-wasm build-server clean run test install

GOPATH_LOCAL := $(shell pwd)/.go
GOMODCACHE_LOCAL := $(shell pwd)/.go/pkg/mod
GOCACHE_LOCAL := $(shell pwd)/.go/cache

all: build-wasm build-server

build-wasm:
	mkdir -p web
	mkdir -p $(GOCACHE_LOCAL)
	GOPATH=$(GOPATH_LOCAL) GOMODCACHE=$(GOMODCACHE_LOCAL) GOCACHE=$(GOCACHE_LOCAL) GOOS=js GOARCH=wasm go build -o web/main.wasm ./cmd/wasm

build-server:
	mkdir -p $(GOCACHE_LOCAL)
	GOPATH=$(GOPATH_LOCAL) GOMODCACHE=$(GOMODCACHE_LOCAL) GOCACHE=$(GOCACHE_LOCAL) go build -o chess-server cmd/server/main.go

clean:
	rm -f web/main.wasm chess-server
	rm -rf .go

run: all
	./chess-server

test:
	GOPATH=$(GOPATH_LOCAL) GOMODCACHE=$(GOMODCACHE_LOCAL) go test ./...

install: all
	./install.sh
