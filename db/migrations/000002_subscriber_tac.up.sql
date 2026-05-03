-- Replace the two free-text device columns with a single 8-digit TAC
-- (Type Allocation Code). The manufacturer/model strings are derived
-- client-side from the built-in TAC catalogue, so they do not need to
-- be persisted.
ALTER TABLE subscriber
    DROP COLUMN IF EXISTS device_make,
    DROP COLUMN IF EXISTS device_model,
    ADD COLUMN  tac text;
