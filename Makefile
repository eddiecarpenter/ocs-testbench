# OCS Testbench — top-level Make targets.
#
# Convention: targets without the `integration` tag are safe to run
# on any developer machine (no Docker required); targets with the tag
# require Docker so testcontainers-go can stand up a Postgres image.

.PHONY: build build-desktop vet test test-integration test-all generate clean

# Version — derived from the nearest git tag. Falls back to "dev" when
# there are no tags (fresh clone) or git is unavailable (CI sandbox).
VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -X main.Version=$(VERSION)

# build — compile every package for headless/server deployment.
# -a forces a full rebuild so that changes to go:embed targets (web/dist)
# are always picked up by the Go build cache.
build:
	go build -a -ldflags "$(LDFLAGS)" -o ocs-testbench ./cmd/ocs-testbench/ && go build ./...

# build-desktop — produce a macOS .app bundle via the Wails toolchain.
# Requires: go install github.com/wailsapp/wails/v2/cmd/wails@latest
# The frontend SPA is built from web/ as part of this step. The bundle
# lands in cmd/ocs-testbench/build/bin/OCS Testbench.app.
# config.yaml is copied next to the binary so the app finds it without
# CONFIG_FILE when launched from Finder.
APP_BIN=cmd/ocs-testbench/build/bin/ocs-testbench.app/Contents/MacOS
build-desktop:
	cd cmd/ocs-testbench && wails build -clean -skipbindings -ldflags "$(LDFLAGS)"
	cp cmd/ocs-testbench/config.yaml "$(APP_BIN)/config.yaml"

# vet — static analysis; runs in CI alongside the build.
vet:
	go vet ./...

# test — fast unit-test pass over every package. Excludes integration
# tests by virtue of their `integration` build tag, so this runs
# without Docker.
test:
	go test ./... -count=1 -timeout 60s

# test-integration — drives the production Store against a real
# PostgreSQL instance via testcontainers-go. Requires Docker.
test-integration:
	go test -tags integration ./internal/store/... -count=1 -timeout 600s

# test-all — runs both passes back-to-back. Useful for local
# verification before pushing.
test-all: test test-integration

# generate — regenerates the sqlc bindings under
# internal/store/sqlc/. Re-run whenever the schema or query files
# change.
generate:
	sqlc generate

# clean — remove transient artefacts. Currently a no-op; reserved for
# future build outputs.
clean:
	@echo "nothing to clean"
