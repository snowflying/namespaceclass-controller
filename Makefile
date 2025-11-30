# Variables
BINARY=bin/controller

# Default target
all: build

# Build the binary
build:
	@echo "Building..."
	# go build -o $(BINARY) ./...

# Run unit tests
test:
	@echo "Running tests..."
	# go test ./...

# Run linter
lint:
	@echo "Linting..."
	# golangci-lint run ./...

# Clean binaries
clean:
	@echo "Cleaning..."
	# rm -rf $(BINARY)
