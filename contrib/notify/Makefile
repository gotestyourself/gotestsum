GO = go
INSTALL = install

ICONS = icons/test-pass.svg icons/test-fail.svg
ICONDIR = $(HOME)/.icons  # or /usr/share/icons

build:
	$(GO) build

install: $(ICONS)
	$(GO) install
	$(INSTALL) -d $(ICONDIR)
	$(INSTALL) $^ $(ICONDIR)
