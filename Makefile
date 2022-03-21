all: build-tunnel build-tunneld

.PHONY: setup
setup:
	mkdir -p build

.PHONY: build-tunnel
build-tunnel:
	cd cmd/tunnel/; go build -o ../../build/ ; cd -

.PHONY: build-tunneld
build-tunneld:
	cd cmd/tunneld/; go build -o ../../build/ ; cd -

.PHONY: test
test:
	go test -v
