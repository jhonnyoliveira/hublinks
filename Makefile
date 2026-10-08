TEST_DATABASE_URL ?= postgresql://hublinks:hublinks@postgres:5432/postgres
export TEST_DATABASE_URL
TAILWIND_VERSION ?= 4.1.17

.PHONY: run test test-race test-ci lint css
run:
	go run ./cmd/hublinks serve
test:
	go test ./...
test-race:
	go test -race ./...
test-ci:
	@output=$$(go test -v ./... 2>&1); status=$$?; printf '%s\n' "$$output"; test $$status -eq 0; ! printf '%s\n' "$$output" | grep -E '^--- SKIP'
lint:
	test -z "$$(gofmt -l .)"
	go vet ./...
css:
	TAILWIND_VERSION=$(TAILWIND_VERSION) scripts/build-css.sh
