BINARY_NAME=stream-watcher

SRC=$(shell find . -name "*.go" -type f)

.PHONY: build build-debug build-linux clean test fmt frontend

build: frontend
	go build -o $(BINARY_NAME) ./cmd

build-debug: frontend
	go build -tags=debug -o $(BINARY_NAME) ./cmd

build-linux: frontend
	GOOS=linux go build -o $(BINARY_NAME) ./cmd

frontend:
	cd helpers/webserver/frontend && yarn install && yarn build

clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -rf helpers/webserver/frontend/dist

test:
	go test -v ./...

fmt:
	gofmt -w $(SRC)
