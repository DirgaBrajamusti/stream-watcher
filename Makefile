# Name of the binary to build
BINARY_NAME=super-bad-stream-watcher

# Go source files
SRC=$(shell find . -name "*.go" -type f)

# Build the binary for the current platform
build:
	go build o $(BINARY_NAME) ./cmd

build-debug:
	go build -tags=debug -o $(BINARY_NAME) ./cmd

build-linux:
	GOOS=linux go build -o $(BINARY_NAME) ./cmd

# Clean the project
clean:
	go clean
	rm -f $(BINARY_NAME)

# Run the tests
test:
	go test -v ./...

# Format the source code
fmt:
	gofmt -w $(SRC)
