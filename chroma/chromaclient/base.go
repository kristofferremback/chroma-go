//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen -package chromaclient -generate types -o ./types.gen.go ./openapi.json
//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen -package chromaclient -generate client -o ./client.gen.go ./openapi.json
package chromaclient
