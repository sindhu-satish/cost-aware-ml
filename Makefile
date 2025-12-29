.PHONY: up down build logs clean

up:
	docker compose up -d --build

down:
	docker compose down

build:
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/controlplane ./cmd/controlplane

logs:
	docker compose logs -f

clean:
	rm -rf bin/
	docker compose down -v
