BINARY := br
INSTALL_PATH := /usr/local/bin/$(BINARY)

.PHONY: build install test vet tidy clean

build:
	go build -o $(BINARY) .

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
