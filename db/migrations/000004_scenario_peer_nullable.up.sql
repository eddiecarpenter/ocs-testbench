-- Make peer_id nullable: a scenario can be saved without assigning a
-- peer. The peer is required only at execution time, not at authoring
-- time. Existing rows are unaffected (non-null values stay non-null).
ALTER TABLE scenario
    ALTER COLUMN peer_id DROP NOT NULL;
