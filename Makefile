openapi-generate:
	./scripts/openapi-generate.sh

go-gen: openapi-generate
	go generate ./...
