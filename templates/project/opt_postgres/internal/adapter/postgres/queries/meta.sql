-- Seed query so `sqlc generate` has something to compile before you add your
-- own. Safe to delete once queries/ holds real queries.

-- name: GetSchemaMeta :one
SELECT * FROM schema_meta WHERE key = $1;
