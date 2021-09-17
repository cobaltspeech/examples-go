# Copyright 2020 Cobalt Speech and Language Inc.

# Needed tools
BINDIR := $(CURDIR)/tmp/bin
LINTER := $(BINDIR)/golangci-lint
LINTER_VERSION := 1.27.0

# Linux vs Darwin detection for the machine on which the build is taking place (not to be used for the build target)
DEV_OS := $(shell uname -s | tr A-Z a-z)

build: build-cubic-example build-diatheke-example

$(LINTER):
	mkdir -p $(BINDIR)
	wget "https://github.com/golangci/golangci-lint/releases/download/v$(LINTER_VERSION)/golangci-lint-$(LINTER_VERSION)-$(DEV_OS)-amd64.tar.gz" -O - | tar -xz -C $(BINDIR) --strip-components=1 --exclude=README.md --exclude=LICENSE

# Run go-fmt and check for differences. Return nonzero if there's a problem.
.PHONY: fmt-check
fmt-check:
	BADFILES=$$(gofmt -l -s -d $$(find . -type f -name '*.go')) && [ -z "$$BADFILES" ] && exit 0

# Run go-fmt and list the differences
.PHONY: fmt-list
fmt-list:
	gofmt -l -s -d $$(find . -type f -name '*.go')

# Run go-fmt and automatically fix issues
.PHONY: fmt
fmt:
	gofmt -s -w $$(find . -type f -name '*.go')

# Run lint checks
.PHONY: lint-check

# The linter can't handle go.mod files in multiple subdirectories,
# so add a new line here whenever there's a new example added
lint-check: $(LINTER)
	cd cubic && $(LINTER) run --deadline=2m
	cd diatheke && $(LINTER) run --deadline=2m
	cd cmdserver && $(LINTER) run --deadline=2m

# Run tests
.PHONY: test
test: 
	cd cmdserver && go test -cover -race ./...

# Build
.PHONY: build-cubic-example
build-cubic-example:
	cd cubic && go mod tidy && go build -o ./bin/transcribe ./cmd

.PHONY: build-diatheke-example
build-diatheke-example:
	cd diatheke && go mod tidy && \
	go build -o ./bin/audio_client ./cmd/audio_client && \
	go build -o ./bin/cli_client ./cmd/cli_client

# Clean
.PHONY: clean
clean:
	rm -rf */bin
