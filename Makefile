BIN      := claude-dashboard
BINDIR   := $(HOME)/.local/bin
UNITDIR  := $(HOME)/.config/systemd/user
UNIT     := claude-dashboard.service

.PHONY: build test install uninstall

build:
	go build -o bin/$(BIN) ./cmd/claude-dashboard

test:
	go test -race ./...

install: build
	install -d $(BINDIR) $(UNITDIR)
	install -m 0755 bin/$(BIN) $(BINDIR)/$(BIN)
	install -m 0644 deploy/$(UNIT) $(UNITDIR)/$(UNIT)
	systemctl --user daemon-reload
	systemctl --user enable --now $(UNIT)

uninstall:
	-systemctl --user disable --now $(UNIT)
	rm -f $(UNITDIR)/$(UNIT) $(BINDIR)/$(BIN)
	systemctl --user daemon-reload
