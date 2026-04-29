-- Queries for the peer table.
--
-- Avoids Postgres-specific operators per the data-model Feature
-- (no @>, no array predicates) so the SQL stays portable to a
-- future SQLite engine swap.

-- name: InsertPeer :one
-- Inserts a peer; the id is generated server-side via the schema's
-- gen_random_uuid() default. Returns the inserted row so the caller
-- can persist the resulting id without a follow-up read.
INSERT INTO peer (name, body)
VALUES ($1, $2)
RETURNING id, name, body, created_at, updated_at;

-- name: GetPeerByID :one
SELECT id, name, body, created_at, updated_at
FROM peer
WHERE id = $1;

-- name: GetPeerByName :one
SELECT id, name, body, created_at, updated_at
FROM peer
WHERE name = $1;

-- name: ListPeers :many
SELECT id, name, body, created_at, updated_at
FROM peer
ORDER BY name;

-- name: UpdatePeer :one
-- updated_at is bumped in SQL so callers cannot accidentally skip it.
-- Per design plan the store layer (not a trigger) maintains updated_at
-- to keep the schema portable to SQLite.
UPDATE peer
SET name = $2,
    body = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, name, body, created_at, updated_at;

-- name: DeletePeer :exec
DELETE FROM peer
WHERE id = $1;
