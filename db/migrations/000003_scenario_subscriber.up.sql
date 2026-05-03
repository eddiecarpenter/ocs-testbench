-- Align the scenario table with the OpenAPI v0.2 design:
--   • template_id was a reference to avp_template but the design
--     moved to embedding avpTree directly in the body JSONB; make it
--     nullable so existing rows and new creates both work.
--   • subscriber_id links a scenario to its test subscriber identity.
ALTER TABLE scenario
    ALTER COLUMN template_id DROP NOT NULL,
    ADD COLUMN subscriber_id uuid REFERENCES subscriber(id) ON DELETE SET NULL;
