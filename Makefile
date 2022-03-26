REPO_PREFIX ?= ghcr.io/jlandowner/
NAME ?= go-tcp-tunnel
VERSION ?= v0.0.1
CHART_VERSION ?= $(VERSION)

all: build

.PHONY: setup
setup:
	mkdir -p bin

.PHONY: build
build:
	CGO_ENABLED=0 go build -o bin/$(NAME) cmd/main.go

.PHONY: test
test:
	go test ./...

.PHONY: docker-image
docker-image:
	DOCKER_BUILDKIT=1 docker build . -t $(REPO_PREFIX)$(NAME):$(VERSION)

# Update version in version.go
.PHONY: update-version
update-version:
ifndef VERSION
	@echo "Usage: make update-version VERSION=v9.9.9"
	@exit 9
else
ifeq ($(shell expr $(VERSION) : '^v[0-9]\+\.[0-9]\+\.[0-9]\+$$'),0)
	@echo "Invalid VERSION '$(VERSION)' Usage: make update-version VERSION=v9.9.9"
	@exit 9
endif
endif
	sed -i.bk -e 's/const version string = "v[0-9]\+.[0-9]\+.[0-9]\+.*"/const version string = "$(VERSION)"/' ./cmd/main.go
	sed -i.bk \
		-e "s/version: [0-9]\+.[0-9]\+.[0-9]\+.*/version: ${CHART_VERSION:v%=%}/" \
		-e "s/appVersion: v[0-9]\+.[0-9]\+.[0-9]\+.*/appVersion: ${VERSION}/" \
		kubernetes/go-tcp-tunnel/Chart.yaml
