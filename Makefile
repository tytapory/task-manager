DB_USER ?= root
DB_PASSWORD ?= password
DB_HOST ?= localhost
DB_PORT ?= 3306
DB_NAME ?= task_manager

DB_URL := "mysql://$(DB_USER):$(DB_PASSWORD)@tcp($(DB_HOST):$(DB_PORT))/$(DB_NAME)"

.PHONY: up down restart build rebuild logs ps clean help generate_doc migrate-up migrate-down migrate-create

up:
	sudo docker compose up -d

down:
	sudo docker compose down

restart:
	sudo docker compose restart

rebuild:
	sudo docker compose up -d --build

logs:
	sudo docker compose logs -f

ps:
	sudo docker compose ps

clean:
	sudo docker compose down -v

generate_doc:
	swag init -g cmd/main.go

migrate-up:
	migrate -path migrations -database $(DB_URL) up

migrate-down:
	migrate -path migrations -database $(DB_URL) down 1

migrate-create:
	@read -p "Enter migration name (e.g. add_users_table): " name; \
	migrate create -ext sql -dir migrations -seq $$name
