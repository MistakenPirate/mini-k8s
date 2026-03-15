-- name: CreateCluster :one
INSERT INTO clusters (name) VALUES ($1) RETURNING *;
-- name: GetCluster :one
SELECT * FROM clusters WHERE id = $1;
-- name: ListClusters :many
SELECT * FROM clusters ORDER BY created_at DESC;
-- name: UpdateClusterStatus :one
UPDATE clusters SET status = $1, updated_at = now() WHERE id = $2 RETURNING *;
-- name: DeleteCluster :exec
DELETE FROM clusters WHERE id = $1;