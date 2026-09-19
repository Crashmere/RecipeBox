.PHONY: build test linux
build:
	npm --prefix web run build
	go build -trimpath -o bin/recipebox ./cmd/recipebox

test:
	go test -race ./...
	go vet ./...
	npm --prefix web test

linux:
	npm --prefix web run build
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o bin/recipebox-linux-amd64 ./cmd/recipebox
