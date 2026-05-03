-- Queries for the scenario table.
--
-- template_id is nullable (see migration 000003): new scenarios do not
-- reference an avp_template; the AVP tree is embedded in body instead.
-- subscriber_id links a scenario to a subscriber identity.
--
-- Column order in every SELECT / RETURNING matches the Scenario struct
-- field order in models.go so sqlc can reuse the base type directly.

-- name: InsertScenario :one
INSERT INTO scenario (name, peer_id, subscriber_id, body)
VALUES ($1, $2, $3, $4)
RETURNING id, name, template_id, peer_id, body, created_at, updated_at, subscriber_id;

-- name: GetScenarioByID :one
SELECT id, name, template_id, peer_id, body, created_at, updated_at, subscriber_id
FROM scenario
WHERE id = $1;

-- name: GetScenarioByName :one
SELECT id, name, template_id, peer_id, body, created_at, updated_at, subscriber_id
FROM scenario
WHERE name = $1;

-- name: ListScenarios :many
SELECT id, name, template_id, peer_id, body, created_at, updated_at, subscriber_id
FROM scenario
ORDER BY name;

-- name: UpdateScenario :one
UPDATE scenario
SET name          = $2,
    peer_id       = $3,
    subscriber_id = $4,
    body          = $5,
    updated_at    = now()
WHERE id = $1
RETURNING id, name, template_id, peer_id, body, created_at, updated_at, subscriber_id;

-- name: DeleteScenario :exec
DELETE FROM scenario
WHERE id = $1;
