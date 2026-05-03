ALTER TABLE scenario
    DROP COLUMN IF EXISTS subscriber_id,
    ALTER COLUMN template_id SET NOT NULL;
