.PHONY: build run serve release test check check-ui package

NODE ?= node
PORT ?=
NO_BROWSER ?= 0

PORT_FLAG = $(if $(strip $(PORT)),--port "$(PORT)")

build:
	go build -trimpath -o workbench ./cmd/workbench

run: build
	./workbench start $(PORT_FLAG) $(if $(filter 1,$(NO_BROWSER)),--no-browser)

serve: build
	./workbench serve $(PORT_FLAG)

release:
	go build -trimpath -ldflags="-s -w" -o workbench ./cmd/workbench

test:
	go test -race ./...

check: test
	go vet ./...
	$(NODE) --check web/static/app.js

check-ui:
	NODE="$(NODE)" ./scripts/test-ui.sh

package: release
	./workbench package
	./workbench package --runtime-only
