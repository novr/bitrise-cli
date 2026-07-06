BINARY := br
INSTALL_PATH := /usr/local/bin/$(BINARY)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/novr/bitrise-cli/internal/api.Version=$(VERSION)

.PHONY: build install test vet tidy clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/br

install: build
	mv $(BINARY) $(INSTALL_PATH)
	@echo "Installed to $(INSTALL_PATH)"

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
