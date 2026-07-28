.PHONY: tidy build run-api run-consumer vet test

tidy:
	go mod tidy

build:
	CGO_ENABLED=0 go build -o bin/news-ingest ./cmd

vet:
	go vet ./...

run-api:
	go run ./cmd serve-api

run-consumer:
	go run ./cmd serve-consumer

test:
	go test ./...
