.PHONY: infra infra-down api frontend migrate-up migrate-down test vet tidy

# Start local Postgres + Redis
infra:
	docker compose up -d

infra-down:
	docker compose down

api:
	cd backend && go run ./cmd/api

frontend:
	cd frontend && npm run dev

migrate-up:
	cd backend && go run ./cmd/migrate up

migrate-down:
	cd backend && go run ./cmd/migrate down 1

test:
	cd backend && go test ./...

vet:
	cd backend && go vet ./...

tidy:
	cd backend && go mod tidy
