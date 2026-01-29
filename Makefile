# Include variables from the .envrc file
include .envrc

.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@printf "Are you sure? [y/n] " && read ans && [ "$${ans:-n}" = y ]

## run/api: run the application
.PHONY: run/api
run/api:
	@go run ./cmd/api -db-dsn=${GREENLIGHT_DB_DSN}

## psql: connect to postgreSQL database via psql
.PHONY: psql
psql:
	@docker compose exec -it database psql -U greenlight

## migrate/create name=$1: create a new database migration
.PHONY: migrate/create
migrate/create:
	@echo 'Creating migration files for ${name}'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

# command with a pre-requisite target
## migrate/up: apply all database migrations
.PHONY: migrate/up
migrate/up: confirm
	@echo 'Running migrations'
	migrate -path ./migrations -database ${GREENLIGHT_DB_DSN} up

#================================================================#
#                    Quality control                             #
#================================================================#

## tidy: tidy module dependecies and format all .go files
.PHONY: tidy
tidy:
	@echo 'Tidying module dependecies...'
	go mod tidy
	@echo 'Verifying and vendoring module dependecies...'
	go mod verify
	go mod vendor
	@echo 'Formatting .go files'
	go fmt ./...


## audit: run quality control checks
.PHONY: audit
audit:
	@echo 'Checking module dependecies...'
	go mod tidy -diff
	go mod verify
	@echo 'Vetting code...'
	go vet ./...
	go tool staticcheck ./...
	@echo 'Running tests...'
	go test -race -vet=off ./...

#================================================================#
#                           Build                                #
#================================================================#

## build/api: build the application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -ldflags='-s' -o=./bin/api ./cmd/api
	GOOS=linux GOARCH=amd64 go build -ldflags='-s' -o=./bin/linux_amd64/api ./cmd/api


#================================================================#
#                      Production                                #
#================================================================#

## production/connect: connect to the production server
.PHONY: production/connect
production/connect:
	ssh greenlight@${PRODUCTION_HOST_IP}


## production/deploy/api: deploy the api to production
.PHONY: production/deploy/api
production/deploy/api:
	rsync -P ./bin/linux_amd64/api greenlight@${PRODUCTION_HOST_IP}:~
	rsync -rP --delete ./migrations greenlight@${PRODUCTION_HOST_IP}:~
	rsync -rP --delete ./remote/production/api.service greenlight@${PRODUCTION_HOST_IP}:~
	rsync -rP --delete ./remote/production/Caddyfile greenlight@${PRODUCTION_HOST_IP}:~
	ssh -t greenlight@${PRODUCTION_HOST_IP} '\
		migrate -path ~/migrations -database $$GREENLIGHT_DB_DSN up \
		&& sudo mv ~/api.service /etc/systemd/system/ \
		&& sudo systemctl enable api \
		&& sudo systemctl restart api \
		&& sed -i "s|{{PRODUCTION_HOST_IP}}|$(PRODUCTION_HOST_IP)|g" ~/Caddyfile \
		&& sudo mv ~/Caddyfile /etc/caddy/ \
		&& sudo systemctl reload caddy \
	'
