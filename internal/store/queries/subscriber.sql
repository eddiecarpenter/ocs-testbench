-- Queries for the subscriber table.
--
-- Insert and Update declare more than four parameters; with
-- query_parameter_limit: 4 in sqlc.yaml the generated bindings will
-- accept an `Arg` struct rather than positional parameters, which
-- keeps the call sites readable.

-- name: InsertSubscriber :one
INSERT INTO subscriber (
    name,
    msisdn,
    iccid,
    imei,
    tac
)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, msisdn, iccid, imei, created_at, updated_at, tac;

-- name: GetSubscriberByID :one
SELECT id, name, msisdn, iccid, imei, created_at, updated_at, tac
FROM subscriber
WHERE id = $1;

-- name: GetSubscriberByName :one
-- Subscribers do not have a unique name constraint at the schema
-- level, so a "by name" lookup may match multiple rows; the store
-- layer chooses the deterministic first match (alphabetical id) for
-- catalogue UX. Tests that need richer semantics should use
-- ListSubscribers and filter on the result.
SELECT id, name, msisdn, iccid, imei, created_at, updated_at, tac
FROM subscriber
WHERE name = $1
ORDER BY id
LIMIT 1;

-- name: ListSubscribers :many
SELECT id, name, msisdn, iccid, imei, created_at, updated_at, tac
FROM subscriber
ORDER BY name;

-- name: UpdateSubscriber :one
UPDATE subscriber
SET name       = $2,
    msisdn     = $3,
    iccid      = $4,
    imei       = $5,
    tac        = $6,
    updated_at = now()
WHERE id = $1
RETURNING id, name, msisdn, iccid, imei, created_at, updated_at, tac;

-- name: DeleteSubscriber :exec
DELETE FROM subscriber
WHERE id = $1;
