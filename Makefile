.PHONY: test build frontend-install frontend-build up down logs backup

test:
	cd backend && go test ./...

build:
	cd backend && CGO_ENABLED=0 go build -o bin/bot ./cmd/bot

frontend-install:
	cd frontend && npm install

frontend-build:
	cd frontend && npm run build

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

backup:
	@mkdir -p backups
	@ts=$$(date -u +%Y%m%dT%H%M%SZ); \
	  docker compose exec -T bot sqlite3 /data/bot.db ".backup /data/bot-$$ts.db" && \
	  docker compose cp bot:/data/bot-$$ts.db backups/bot-$$ts.db && \
	  echo "wrote backups/bot-$$ts.db"
