.PHONY: test clean fmt vet check all lint modernize build

# Find all Go source files
GO_FILES := $(shell find . -name '*.go' -type f)

# Build the cherry-picker binary
cherry-picker: check $(GO_FILES)
	go build -o cherry-picker .

# Build the dep-merger binary
dep-merger: check $(GO_FILES)
	go build -o dep-merger ./cmd/dep-merger

# Build both binaries
build: cherry-picker dep-merger

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -cover ./...

# Clean build artifacts
clean:
	rm -f cherry-picker dep-merger

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run all checks (unit tests only for speed)
check: fmt vet test lint modernize

# Default target
all: check build

lint: modernize fmt
	golangci-lint run \
		--config=.golangci.yaml \
		--timeout=5m \
	    ./...

modernize:
	modernize ./...
