# SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-License-Identifier: EUPL-1.2

GITFLOWER_BIN ?= $(shell pwd)/gitflower

.PHONY: build test e2e e2e-format lint clean help

build:
	go build -o $(GITFLOWER_BIN) .

# `go test ./...` covers:
#   - review/: format Render/Parse round-trip
#   - tui/:    bubbletea Model driven via synthetic KeyPressMsgs
#   - test/e2e/: real binary spawned under a PTY against the fixture repo,
#                walked through a scripted keystroke scenario, then the
#                produced .review is asserted.
test:
	go test ./...

# Format-pipeline smoke check: builds the binary, rebuilds the fixture
# repo via setup.sh, runs `gitflower review --no-tui`, then diffs the
# resulting .review against the golden after normalising dates/SHAs.
e2e-format: build
	./test/e2e/smoke.sh

# Convenience: run every test layer in one go.
e2e: test e2e-format

lint:
	go vet ./...

clean:
	rm -f $(GITFLOWER_BIN)
	rm -rf /tmp/gitflower-e2e-repo

help:
	@echo 'Targets:'
	@echo '  build       compile the gitflower binary'
	@echo '  test        run all Go tests (unit, TUI integration, PTY e2e)'
	@echo '  e2e-format  format pipeline smoke check (build + scaffold + golden diff)'
	@echo '  e2e         everything above'
	@echo '  lint        go vet'
	@echo '  clean       remove the binary and the test repo'
