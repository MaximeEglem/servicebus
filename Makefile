BINARY = sb
MODULE = $(shell go list -m)

.PHONY: build build-all test clean

build:
	go build -o $(BINARY) ./cmd/sb

build-all:
	mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -o dist/$(BINARY)-linux-amd64   ./cmd/sb
	GOOS=darwin  GOARCH=amd64 go build -o dist/$(BINARY)-darwin-amd64  ./cmd/sb
	GOOS=darwin  GOARCH=arm64 go build -o dist/$(BINARY)-darwin-arm64  ./cmd/sb
	GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY)-windows-amd64.exe ./cmd/sb

test:
	go test ./...

clean:
	rm -f $(BINARY)
	rm -rf dist/

build-ffi-mac:
	go build -buildmode=c-shared -o dist/libsb.dylib ./pkg/ffi/

build-ffi-linux:
	go build -buildmode=c-shared -o dist/libsb.so ./pkg/ffi/
