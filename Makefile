GO ?= go
GOFMT ?= gofmt "-s"
GOFILES := $(shell find . -name "*.go")

all: build

.PHONY: build
build: 
	$(GO) mod tidy
	$(GO) build -o bin/simple example/simple/simple.go
	$(GO) build -o bin/timed example/timed/timed.go
	$(GO) build -o bin/multiclient example/multiclient/multiclient.go

.PHONY: test
test: 
	$(GO) test -v

.PHONY: examples
examples: build 
	./bin/simple
	./bin/timed
	./bin/multiclient

.PHONY: fmt
fmt:
	$(GOFMT) -w $(GOFILES)

.PHONY: fmt-check
fmt-check:
	@diff=$$($(GOFMT) -d $(GOFILES)); \
  if [ -n "$$diff" ]; then \
    echo "Please run 'make fmt' and commit the result:"; \
    echo "$${diff}"; \
    exit 1; \
  fi;
