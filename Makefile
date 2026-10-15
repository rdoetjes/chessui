.PHONY: all build-wasm build-server clean run test install

# Use shell-level PWD to ensure compatibility between GNU Make (macOS) and BSD Make (FreeBSD)
GOPATH_LOCAL = ./.go
GOMODCACHE_LOCAL = ./.go/pkg/mod
GOCACHE_LOCAL = ./.go/cache

all: build-wasm build-server

build-wasm:
	mkdir -p web
	mkdir -p $(GOCACHE_LOCAL)
	GOPATH=$$(pwd)/.go GOMODCACHE=$$(pwd)/.go/pkg/mod GOCACHE=$$(pwd)/.go/cache GOOS=js GOARCH=wasm go build -o web/main.wasm ./cmd/wasm

build-server:
	mkdir -p $(GOCACHE_LOCAL)
	GOPATH=$$(pwd)/.go GOMODCACHE=$$(pwd)/.go/pkg/mod GOCACHE=$$(pwd)/.go/cache go build -o chess-server cmd/server/main.go

clean:
	rm -f web/main.wasm chess-server
	rm -rf .go

run: all
	./chess-server

test:
	GOPATH=$$(pwd)/.go GOMODCACHE=$$(pwd)/.go/pkg/mod go test ./...

install: all
	./install.sh
