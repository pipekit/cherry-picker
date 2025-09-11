.PHONY: test clean fmt vet

# Find all Go source files
GO_FILES := $(shell find . -name '*.go' -type f)

# Build the cherry-picker binary
cherry-picker: check $(GO_FILES)
	go build -o cherry-picker .

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f cherry-picker

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run all checks
check: fmt vet test

# Default target
all: check build
