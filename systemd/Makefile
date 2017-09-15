.PHONY: install uninstall

install:
	install -Dm0644 pofwd.service /usr/lib/systemd/system/
	touch /etc/pofwd.conf
	systemctl daemon-reload || true

uninstall:
	rm -f /usr/lib/systemd/system/pofwd.service /etc/pofwd.conf
	systemctl daemon-reload || true
