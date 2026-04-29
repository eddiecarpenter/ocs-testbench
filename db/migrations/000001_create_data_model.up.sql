-- 000001_create_data_model.up.sql
--
-- Bootstraps the OCS Testbench data model: peer, subscriber,
-- avp_template, scenario, and custom_dictionary. Single migration is
-- deliberate (see Feature #16 design plan, "Key Decisions"): the
-- tables are introduced as a coherent unit and the foreign keys need
-- both sides present at the time the constraints are declared.
--
-- Compatibility constraint: avoid PostgreSQL-specific operators (no
-- @>, no array types, no JSONB containment in DDL) so the schema
-- can be ported to SQLite later. JSONB columns are kept; the SQLite
-- equivalent is TEXT and the column-level operations performed by
-- the application stay portable.

-- pgcrypto powers gen_random_uuid() for default primary keys. CREATE
-- EXTENSION IF NOT EXISTS is idempotent under repeated migration runs.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- peer
-- ---------------------------------------------------------------------------
-- A configured Diameter peer. The body column carries the full
-- structured peer configuration (identity, connection, CER capabilities,
-- watchdog, startup) as JSONB so the catalogue can evolve without
-- schema churn. The name column is unique so peers can be referenced
-- by a stable human-friendly identifier.
CREATE TABLE peer (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL UNIQUE,
    body       jsonb       NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- subscriber
-- ---------------------------------------------------------------------------
-- A subscriber identity used to drive scenarios. Functional identifiers
-- (msisdn, iccid) are mandatory; device identifiers (imei, device make
-- and model) are optional so partially-known subscribers can still be
-- catalogued. The display name is mandatory to keep the catalogue UX
-- usable.
CREATE TABLE subscriber (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name         text        NOT NULL,
    msisdn       text        NOT NULL,
    iccid        text        NOT NULL,
    imei         text,
    device_make  text,
    device_model text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- avp_template
-- ---------------------------------------------------------------------------
-- A reusable AVP tree (typically a CCR shape with MSCC blocks). The body
-- carries the full AVP structure as JSONB so MSCC fields and other
-- nested structures round-trip without schema churn. Templates are
-- referenced by scenarios via foreign key.
CREATE TABLE avp_template (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL UNIQUE,
    body       jsonb       NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- scenario
-- ---------------------------------------------------------------------------
-- A scenario binds an AVP template and a peer with a JSONB body that
-- captures variables, services, and the ordered step list (steps may
-- contain extractions, guards, and assertions per the design plan).
--
-- ON DELETE RESTRICT is deliberate (design plan, Key Decisions):
-- removing a template or peer that a scenario depends on is almost
-- certainly a user mistake and should surface an error rather than
-- silently breaking scenarios.
CREATE TABLE scenario (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text        NOT NULL UNIQUE,
    template_id uuid        NOT NULL REFERENCES avp_template (id) ON DELETE RESTRICT,
    peer_id     uuid        NOT NULL REFERENCES peer (id)         ON DELETE RESTRICT,
    body        jsonb       NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- custom_dictionary
-- ---------------------------------------------------------------------------
-- A user-supplied Diameter dictionary fragment. The xml_content column
-- holds the raw XML payload; is_active flips the dictionary in or out
-- of the active set without deletion.
CREATE TABLE custom_dictionary (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text        NOT NULL UNIQUE,
    description text,
    xml_content text        NOT NULL,
    is_active   boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
