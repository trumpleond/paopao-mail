.PHONY: run build tidy test dist

APP=paopao-api
VERSION?=$(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS=-s -w -X main.version=$(VERSION) -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)

run:
	go run ./cmd/server

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(APP) ./cmd/server

# Cross-compile common targets (CGO-free sqlite)
dist:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(APP)-linux-amd64 ./cmd/server
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(APP)-linux-arm64 ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(APP)-windows-amd64.exe ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(APP)-windows-arm64.exe ./cmd/server
	CGO_ENABLED=0 GOOS=linux   GOARCH=386   go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(APP)-linux-386 ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=386   go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(APP)-windows-386.exe ./cmd/server
	@echo "built:" && ls -la dist/

tidy:
	go mod tidy

test:
	go test ./...
