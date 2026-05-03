ALTER TABLE subscriber
    DROP COLUMN IF EXISTS tac,
    ADD COLUMN device_make  text,
    ADD COLUMN device_model text;
