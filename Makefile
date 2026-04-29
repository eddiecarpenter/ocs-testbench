# OCS Testbench — top-level Make targets.
#
# Convention: targets without the `integration` tag are safe to run
# on any developer machine (no Docker required); targets with the tag
# require Docker so testcontainers-go can stand up a Postgres image.

.PHONY: build vet test test-integration test-all generate clean

# build — compile every package; the canonical pre-commit gate.
build:
	go build ./...

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
