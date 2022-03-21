BINARY = tcptunnel

all: build

.PHONY: setup
setup:
	mkdir -p bin

.PHONY: build
build:
	go build -o bin/$(BINARY) cmd/main.go

.PHONY: test
test:
	go test -v
