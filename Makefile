.PHONY: up down build logs clean test load

up:
	docker compose up -d --build

down:
	docker compose down

build:
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/controlplane ./cmd/controlplane
	go build -o bin/simulator ./cmd/simulator

logs:
	docker compose logs -f

clean:
	rm -rf bin/
	docker compose down -v

test:
	go test ./pkg/... -v

load:
	docker compose run --rm simulator
