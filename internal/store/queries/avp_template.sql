-- Queries for the avp_template table.

-- name: InsertAVPTemplate :one
INSERT INTO avp_template (name, body)
VALUES ($1, $2)
RETURNING id, name, body, created_at, updated_at;

-- name: GetAVPTemplateByID :one
SELECT id, name, body, created_at, updated_at
FROM avp_template
WHERE id = $1;

-- name: GetAVPTemplateByName :one
SELECT id, name, body, created_at, updated_at
FROM avp_template
WHERE name = $1;

-- name: ListAVPTemplates :many
SELECT id, name, body, created_at, updated_at
FROM avp_template
ORDER BY name;

-- name: UpdateAVPTemplate :one
UPDATE avp_template
SET name = $2,
    body = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, name, body, created_at, updated_at;

-- name: DeleteAVPTemplate :exec
DELETE FROM avp_template
WHERE id = $1;
