.PHONY: build test clean install

BINARY := ishtrak
BUILD_DIR := dist

build:
	go build -o $(BUILD_DIR)/$(BINARY) .

test:
	go test ./...

install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
	@echo "Installed: /usr/local/bin/$(BINARY)"
	@echo "Run: ishtrak init"

clean:
	rm -rf $(BUILD_DIR)

lint:
	go vet ./...
