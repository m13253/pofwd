.PHONY: all clean install uninstall

all: pofwd

PREFIX=/usr/local

clean:
	rm -f pofwd

install: all
	install -Dm0755 pofwd "$(PREFIX)/bin/pofwd"
	$(MAKE) -C systemd install

uninstall:
	rm -f "$(PREFIX)/bin/pofwd"
	$(MAKE) -C systemd uninstall

pofwd: pofwd.go
	go build -o pofwd pofwd.go
