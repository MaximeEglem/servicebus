BINARY = sb
MODULE = $(shell go list -m)
VERSION ?= $(shell sh -c 'desc=$$(git describe --tags --dirty --always --match "v[0-9]*" 2>/dev/null || true); if [ -n "$$desc" ] && printf "%s" "$$desc" | grep -Eq "^v[0-9]"; then printf "%s" "$$desc"; else commit=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown); printf "v0.0.0-untagged+%s" "$$commit"; fi')
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -X main.version=$(VERSION) -X main.gitCommit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: build build-all test clean release-patch release-minor release-major

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/sb

build-all:
	mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64   ./cmd/sb
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64  ./cmd/sb
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64  ./cmd/sb
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./cmd/sb

test:
	go test ./...

clean:
	rm -f $(BINARY)
	rm -rf dist/

build-ffi-mac:
	go build -buildmode=c-shared -o dist/libsb.dylib ./pkg/ffi/

build-ffi-linux:
	go build -buildmode=c-shared -o dist/libsb.so ./pkg/ffi/

release-patch:
	@./scripts/release.sh patch

release-minor:
	@./scripts/release.sh minor

release-major:
	@./scripts/release.sh major
