// Package docs 提供编译进服务的 OpenAPI 接口契约。
package docs

import _ "embed"

// OpenAPISpec 是 FollowingFeed 的 OpenAPI 3.0 文档。
//
//go:embed api.yaml
var OpenAPISpec []byte
