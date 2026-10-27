.PHONY: build run serve release test check package package-npm check-npm

NODE ?= node
PORT ?=
NO_BROWSER ?= 0
VERSION ?= dev

PORT_FLAG = $(if $(strip $(PORT)),--port "$(PORT)")

build:
	go build -trimpath -o workbench ./cmd/workbench

run: build
	./workbench start $(PORT_FLAG) $(if $(filter 1,$(NO_BROWSER)),--no-browser)

serve: build
	./workbench serve $(PORT_FLAG)

release:
	go build -trimpath -ldflags="-s -w -X workbench/internal/app.Version=$(VERSION)" -o workbench ./cmd/workbench

test:
	go test -race ./...

check: test
	go vet ./...
	$(NODE) scripts/check-frontend.cjs
	$(NODE) --test npm/*.test.cjs

package: release
	./workbench package
	./workbench package --runtime-only

package-npm:
	$(NODE) scripts/build-npm.cjs

check-npm:
	$(NODE) scripts/check-npm.cjs
