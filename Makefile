CWD = $(shell pwd)
SRC_DIRS := .
BUILD_VERSION=$(shell cat config.json | awk 'BEGIN { FS="\""; RS="," }; { if ($$2 == "version") {print $$4} }')
REPO=danielapatin/go-parking

.PHONY: build publish

build:
	@BUILD_VERSION=$(BUILD_VERSION) KO_DOCKER_REPO=$(REPO) ko build ./cmd/go-parking --bare --local --tags="$(BUILD_VERSION),latest"

publish:
	@BUILD_VERSION=$(BUILD_VERSION) KO_DOCKER_REPO=$(REPO) ko publish ./cmd/go-parking --bare --tags="$(BUILD_VERSION),latest"

lint:
	@golangci-lint run -v

test:
	@chmod +x ./test.sh
	@./test.sh $(SRC_DIRS)
