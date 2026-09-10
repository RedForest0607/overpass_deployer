package migrations

import "embed"

// Files contains the ordered, immutable SQLite migrations applied at startup.
//
//go:embed *.sql
var Files embed.FS
