.PHONY: build build-tracker-transition test-go

build: build-tracker-transition

build-tracker-transition:
	mkdir -p bin
	go build -o bin/tracker-transition ./skills/yandex-tracker-commenter/cmd/tracker-transition

test-go:
	go test ./...
