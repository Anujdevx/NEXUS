// Package migrations embeds the SQL migrations run on boot.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
