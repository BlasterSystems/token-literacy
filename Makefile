GO ?= go
BINARY := bin/tokenledger
VERSION ?= 0.1.0-dev
COVER_THRESHOLD ?= 75

.PHONY: build test cover run clean

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BINARY) ./cmd/tokenledger

test:
	$(GO) test ./... -count=1

# Test pyramid: unit + integration tests everywhere, one e2e pass in cmd,
# total coverage gated at COVER_THRESHOLD.
cover:
	$(GO) test ./... -count=1 -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out | tee coverage.txt | tail -1
	@awk '/^total:/ { gsub("%", "", $$3); if ($$3 + 0 < $(COVER_THRESHOLD)) { print "FAIL: total coverage " $$3 "% is below $(COVER_THRESHOLD)%"; exit 1 } }' coverage.txt

run:
	$(GO) run ./cmd/tokenledger serve

clean:
	rm -rf bin coverage.out coverage.txt
