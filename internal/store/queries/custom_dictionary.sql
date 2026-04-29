-- Queries for the custom_dictionary table.
--
-- Insert and Update have five parameters; with query_parameter_limit: 4
-- the generated bindings accept a struct rather than positional
-- parameters.

-- name: InsertCustomDictionary :one
INSERT INTO custom_dictionary (
    name,
    description,
    xml_content,
    is_active
)
VALUES ($1, $2, $3, $4)
RETURNING id, name, description, xml_content, is_active, created_at, updated_at;

-- name: GetCustomDictionaryByID :one
SELECT id, name, description, xml_content, is_active, created_at, updated_at
FROM custom_dictionary
WHERE id = $1;

-- name: GetCustomDictionaryByName :one
SELECT id, name, description, xml_content, is_active, created_at, updated_at
FROM custom_dictionary
WHERE name = $1;

-- name: ListCustomDictionaries :many
SELECT id, name, description, xml_content, is_active, created_at, updated_at
FROM custom_dictionary
ORDER BY name;

-- name: UpdateCustomDictionary :one
UPDATE custom_dictionary
SET name        = $2,
    description = $3,
    xml_content = $4,
    is_active   = $5,
    updated_at  = now()
WHERE id = $1
RETURNING id, name, description, xml_content, is_active, created_at, updated_at;

-- name: DeleteCustomDictionary :exec
DELETE FROM custom_dictionary
WHERE id = $1;
