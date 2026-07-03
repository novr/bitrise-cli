BINARY := br
INSTALL_PATH := /usr/local/bin/$(BINARY)

.PHONY: build install clean tidy

build:
	go build -o $(BINARY) .

install: build
	mv $(BINARY) $(INSTALL_PATH)
	@echo "Installed to $(INSTALL_PATH)"

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
