.PHONY: test test-integration test-all clean fmt vet check all lint modernize

# Find all Go source files
GO_FILES := $(shell find . -name '*.go' -type f)

# Build the cherry-picker binary
cherry-picker: check $(GO_FILES)
	go build -o cherry-picker .

# Run unit tests only (excludes integration tests)
test:
	go test -v ./...

# Run integration tests only
test-integration:
	go test -tags=integration -v ./...

# Run all tests (unit + integration)
test-all:
	go test -tags=integration -v ./...

# Run tests with coverage
test-coverage:
	go test -cover ./...

# Run all tests with coverage (including integration)
test-coverage-all:
	go test -tags=integration -cover ./...

# Clean build artifacts
clean:
	rm -f cherry-picker

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
