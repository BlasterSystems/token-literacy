GO ?= go
BINARY := bin/tokenledger
VERSION ?= 0.1.0-dev

.PHONY: build test run clean

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BINARY) ./cmd/tokenledger

test:
	$(GO) test ./...

run:
	$(GO) run ./cmd/tokenledger serve

clean:
	rm -rf bin
