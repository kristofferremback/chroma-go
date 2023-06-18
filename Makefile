chroma/chromaclient/openapi.json:
	./scripts/openapi-generate.sh

go-gen: chroma/chromaclient/openapi.json
	go generate ./...
