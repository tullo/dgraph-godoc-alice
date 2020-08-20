.PHONY: help
help:
	@go run .

schema:
	@go run . schema

mutate:
	@go run . mutate

query:
	@go run . query

upsert:
	@go run . upsert

drop-data:
	@go run . drop-data

drop-schema:
	@go run . drop-schema

deps-upgrade:
	@go get -u -t -d -v ./...
	@go mod tidy

dgraph-down:
	@docker-compose down --remove-orphans --volumes

dgraph-logs:
	@docker-compose logs -f --tail="10"
	
dgraph-up:
	@docker-compose up --detach --remove-orphans

ratel-ui:
	@firefox http://localhost:8000/?local

.PHONY: check
check:
	$(shell go env GOPATH)/bin/staticcheck -go 1.15 -tests ./...

.PHONY: clone
clone:
	@git clone git@github.com:dominikh/go-tools.git /tmp/go-tools \
		&& cd /tmp/go-tools \
		&& git checkout "2020.1.5" \

.PHONY: install
install:
	@cd /tmp/go-tools && go install -v ./cmd/staticcheck
	$(shell go env GOPATH)/bin/staticcheck -debug.version
