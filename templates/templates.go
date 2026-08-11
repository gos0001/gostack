package templates

import "embed"

// The all: prefix is required — without it Go skips files and directories whose
// names begin with "." or "_" anywhere in the tree. That would drop
// .env.development.tmpl, .gitignore.tmpl, the whole .claude/ skill tree that
// every generated project ships with, and any page directory named _post_id.
//
//go:embed all:project all:usecase all:api all:page all:page_tmpl all:crud all:orchestrator
var FS embed.FS
