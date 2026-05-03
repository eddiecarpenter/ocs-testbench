-- Reverse: restore peer_id NOT NULL. This will fail if any row has a
-- null peer_id — clean those up before rolling back.
ALTER TABLE scenario
    ALTER COLUMN peer_id SET NOT NULL;
