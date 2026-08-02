// Package web nhúng toàn bộ frontend tĩnh vào binary.
package web

import "embed"

//go:embed all:static
var FS embed.FS
