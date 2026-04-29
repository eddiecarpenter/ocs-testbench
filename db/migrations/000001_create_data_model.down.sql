-- 000001_create_data_model.down.sql
--
-- Reverses 000001_create_data_model.up.sql. Tables are dropped in
-- reverse foreign-key order: scenario depends on avp_template and
-- peer, so it goes first. The other tables are independent of each
-- other and follow.
--
-- The pgcrypto extension is intentionally NOT dropped — other
-- migrations or other applications sharing the database may rely on
-- gen_random_uuid().

DROP TABLE IF EXISTS scenario;
DROP TABLE IF EXISTS custom_dictionary;
DROP TABLE IF EXISTS subscriber;
DROP TABLE IF EXISTS avp_template;
DROP TABLE IF EXISTS peer;
