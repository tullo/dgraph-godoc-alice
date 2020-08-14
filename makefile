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
