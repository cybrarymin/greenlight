## help: print the help message
-include .envrc # -include will include .envrc but if it doesn't exist it won't return error. .envrc usually is not commited in git so to avoid pipeline failure we do this

#================================================================#
# HELPERS
#================================================================#

# always use helo as the first target. Because make command without any target will run first target defined in it. "make" will equal to "make help"
.PHONY: help # .PHONY for each target will teach make if we have a local file or directory that names help pls don't consider them and use the target we defined cause make command can't dinstingush the directory or file from targets we define inside makefile and it get's confused
help: # @ before the command will not echo the command itself when we run make <target> command
	@echo "Usage:" 
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.PHONY: prerequsite_confirm
prerequsite_confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]



#================================================================#
# DEVELOPMENT
#================================================================#
## build/api: building the application
.PHONY: build/api
build/api:
	@go mod tidy
	@GOOS=linux GOARCH=amd64 go build -o=./bin/greenlight-linux-amd64 ./
	@GOOS=darwin GOARCH=arm64 go build -o=./bin/greenlight-darwin-arm64 ./
	@go build -o=./bin/greenlight-local-compatible ./

## run/api: run the application
.PHONY: run/api
run/api:
	@go run main.go --db-connection-string="postgres://postgres:m.amin24242@localhost:5432/greenlight?sslmode=disable" --smtp-server-addr="sandbox.smtp.mailtrap.io" --smtp-username="16280f8e9645e4" --smtp-password="7a615205806af2"

## /db/migrations/up: running database migrations to create table and indexes
.PHONY: db/migrations/up
db/migrations/up: prerequsite_confirm
	@echo "Running database migrations..."
	migrate -path=./migrations -database ${DATABASE_DSN} up

## db/migrations/create migration_name=<NAME_OF_THE_MIGRATION>: creating new migration file 
.PHONY: db/migrations/create
db/migrations/create:
	@echo "Create a new sequenced migration...."
	migrate create -dir=./migrations -ext=.sql -seq ${migration_name} # migration make command argument


#================================================================#
# QUALITY CHECK , LINTING, Vendoring
#================================================================#
.PHONY: audit
audit:
	@echo "Tidying and verifying golang packages and module dependencies..."
	go mod tidy
	go mod verify
	@echo "Formatting codes..."
	go fmt ./...
	@echo "Vetting codes..."
	go vet ./...
	@echo "Static Checking of the code..."
	staticcheck ./...
	@echo "Running tests..."
	go test -race -vet=off ./...

.PHONY: vendor
vendor:
	@echo "Tidying and verifying golang packages and module dependencies..."
	go mod verify
	go mod tidy
	@echo "Vendoring all golang dependency modules and packages..."
	go mod vendor
