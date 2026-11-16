.PHONY: run build test test-race docker-up docker-down clean

run:
	go run cmd/server/main.go

build:
	go build -v -o bin/server.exe cmd/server/main.go

test:
	go test -v ./...

test-race:
	go test -v -race ./...

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

clean:
	rm -rf bin/
