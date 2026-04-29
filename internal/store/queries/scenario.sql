-- Queries for the scenario table.
--
-- Insert and Update have five parameters; with query_parameter_limit: 4
-- the generated bindings accept a struct rather than positional
-- parameters, keeping call sites readable.

-- name: InsertScenario :one
-- The schema declares ON DELETE RESTRICT foreign keys against
-- avp_template and peer. An insert with a non-existent template_id or
-- peer_id therefore fails with a Postgres FK error (verified in the
-- AC-5 integration test).
INSERT INTO scenario (
    name,
    template_id,
    peer_id,
    body
)
VALUES ($1, $2, $3, $4)
RETURNING id, name, template_id, peer_id, body, created_at, updated_at;

-- name: GetScenarioByID :one
SELECT id, name, template_id, peer_id, body, created_at, updated_at
FROM scenario
WHERE id = $1;

-- name: GetScenarioByName :one
SELECT id, name, template_id, peer_id, body, created_at, updated_at
FROM scenario
WHERE name = $1;

-- name: ListScenarios :many
SELECT id, name, template_id, peer_id, body, created_at, updated_at
FROM scenario
ORDER BY name;

-- name: UpdateScenario :one
UPDATE scenario
SET name        = $2,
    template_id = $3,
    peer_id     = $4,
    body        = $5,
    updated_at  = now()
WHERE id = $1
RETURNING id, name, template_id, peer_id, body, created_at, updated_at;

-- name: DeleteScenario :exec
DELETE FROM scenario
WHERE id = $1;
