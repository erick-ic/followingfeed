.PHONY: up down demo test fmt vet web-build migrate-up
up:
	docker compose up --build
down:
	docker compose down
demo:
	docker compose up --build
test:
	go test ./...
fmt:
	go fmt ./...
vet:
	go vet ./...
web-build:
	cd web && npm run build
migrate-up:
	go run ./cmd/migrate
