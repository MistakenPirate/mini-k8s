-- name: CreatePod :one
INSERT INTO pods (cluster_id, node_id, name, image, cpu_request, memory_request)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: GetPod :one
SELECT * FROM pods WHERE id = $1;

-- name: ListPodsByCluster :many
SELECT * FROM pods WHERE cluster_id = $1 ORDER BY created_at DESC;

-- name: ListPodsByStatus :many
SELECT * FROM pods WHERE cluster_id = $1 AND status = $2 ORDER BY created_at DESC;

-- name: ListPodsByNode :many
SELECT * FROM pods WHERE node_id = $1 ORDER BY created_at DESC;

-- name: AssignPodToNode :one
UPDATE pods SET node_id = $1, status = 'scheduled', updated_at = now() WHERE id = $2 RETURNING *;

-- name: UpdatePodStatus :one
UPDATE pods SET status = $1, updated_at = now() WHERE id = $2 RETURNING *;

-- name: DeletePod :exec
DELETE FROM pods WHERE id = $1;
