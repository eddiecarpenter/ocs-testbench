-- name: ListAIPermissions :many
SELECT tool_name, decision, updated_at FROM ai_tool_permissions ORDER BY tool_name;

-- name: UpsertAIPermission :exec
INSERT INTO ai_tool_permissions (tool_name, decision, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (tool_name) DO UPDATE
    SET decision = EXCLUDED.decision, updated_at = NOW();

-- name: DeleteAIPermission :exec
DELETE FROM ai_tool_permissions WHERE tool_name = $1;
