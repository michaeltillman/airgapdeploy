BINARY := airgapdeploy
PKG := ./cmd/airgapdeploy
VERSION := 0.1.0
LDFLAGS := -s -w

.PHONY: all build run generate vet fmt test dist clean tidy vendor

all: build

## build: compile the single binary into bin/
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

## run: run locally with live execution enabled
run:
	go run $(PKG) --live

## generate: render artifacts headlessly from a config file (CFG=path)
generate:
	go run $(PKG) --generate --config $(CFG)

## vet: static analysis
vet:
	go vet ./...

## fmt: format all Go files
fmt:
	gofmt -w .

## test: run unit tests
test:
	go test ./...

## dist: cross-compile release binaries into dist/
dist:
	@mkdir -p dist
	GOOS=linux  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64   $(PKG)
	GOOS=linux  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64   $(PKG)
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64  $(PKG)
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64  $(PKG)
	@echo "built:" && ls -1 dist/

## tidy: sync go.mod/go.sum
tidy:
	go mod tidy

## vendor: vendor dependencies so the repo builds fully offline
vendor:
	go mod vendor

## clean: remove build output
clean:
	rm -rf bin dist
