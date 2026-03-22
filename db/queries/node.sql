-- name: CreateNode :one
INSERT INTO nodes (cluster_id, name, cpu_millis, memory_mb)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetNode :one
SELECT * FROM nodes WHERE id = $1;

-- name: GetNodeByName :one
SELECT * FROM nodes WHERE cluster_id = $1 AND name = $2;

-- name: ListNodesByCluster :many
SELECT * FROM nodes WHERE cluster_id = $1 ORDER BY created_at DESC;

-- name: UpdateNodeStatus :one
UPDATE nodes SET status = $1, updated_at = now() WHERE id = $2 RETURNING *;

-- name: DeleteNode :exec
DELETE FROM nodes WHERE id = $1;
