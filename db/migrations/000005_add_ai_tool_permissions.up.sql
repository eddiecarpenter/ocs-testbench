CREATE TABLE IF NOT EXISTS ai_tool_permissions (
    tool_name   TEXT        NOT NULL PRIMARY KEY,
    decision    TEXT        NOT NULL CHECK (decision IN ('allow', 'deny')),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
