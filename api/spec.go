// Package api embeds the OpenAPI document so read-api can serve it at
// /v1/openapi.yaml without reading the filesystem.
package api

import _ "embed"

// OpenAPI is the bytes of api/openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPI []byte
