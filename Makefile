.PHONY: build run test ingest eval docker

build:
	go build -o bin/server ./cmd/server
	go build -o bin/ingest ./cmd/ingest
	go build -o bin/eval ./cmd/eval

run:
	set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/server

test:
	go test ./...

ingest:
	go run ./cmd/ingest -dir $(DIR)

eval:
	go run ./cmd/eval -dataset $(or $(DATASET),examples/dataset.example.json) -v

docker:
	docker build -t hybrid-rag-sop-search .
