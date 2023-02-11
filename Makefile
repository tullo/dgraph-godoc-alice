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

staticcheck:
	$$(go env GOPATH)/bin/staticcheck -go 1.20 -tests ./...

staticcheck-install:
	@GO111MODULE=on go install honnef.co/go/tools/cmd/staticcheck@v0.4.0
	@$$(go env GOPATH)/bin/staticcheck -debug.version
