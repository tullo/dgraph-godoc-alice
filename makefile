.PHONY: help
help:
	@go run .

schema:
	@go run . schema

mutate:
	@go run . mutate

query:
	@go run . query

drop-data:
	@go run . drop-data

drop-schema:
	@go run . drop-schema

dgraph-down:
	@docker-compose down --remove-orphans --volumes

dgraph-logs:
	@docker-compose logs -f --tail="10"
	
dgraph-up:
	@docker-compose up --detach --remove-orphans
