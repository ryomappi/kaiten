BIN     := kaiten
PREFIX  ?= $(HOME)/.local

.PHONY: build install uninstall lint clean

build:
	go build -o $(BIN) .

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BIN) $(PREFIX)/bin/$(BIN)

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)

lint:
	go vet ./...

clean:
	rm -f $(BIN)
	rm -rf dist/
