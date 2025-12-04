# Variables
BINARY=controller

# Default target
all: build

# Build the binary
build:
	@echo "Building..."
	go mod download
	go mod tidy
	go build -o $(BINARY) ./...

# Run unit tests
test:
	@echo "Running tests..."
	go test ./... -v

# Run linter
lint:
	@echo "Linting..."
	golangci-lint run ./...

# Clean binaries
clean:
	@echo "Cleaning..."
	rm -rf $(BINARY)
