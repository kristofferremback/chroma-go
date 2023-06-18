//go:build tools

//nolint:typecheck
package tools

import (
	// used for code generation in go:generate directive, must be
	// imported like this so that go mod tidy works as expected.
	_ "github.com/deepmap/oapi-codegen/cmd/oapi-codegen"
)
