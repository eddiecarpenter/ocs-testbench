//go:build integration

// Integration tests for the production Store implementation.
//
// These tests stand up a real PostgreSQL instance via
// testcontainers-go, run the golang-migrate up migrations against it,
// and exercise every Feature #16 acceptance criterion that requires
// actual database semantics (JSONB binary fidelity, schema-level
// constraints, foreign-key enforcement). They are gated by the
// `integration` build tag so a host without Docker can still run
// `go test ./...` and pass.
//
// Run with:
//   go test -tags integration ./internal/store/...
//
// The Makefile exposes `make test-integration` for convenience and CI.

package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// migrationsPath returns an absolute filesystem path to the
// db/migrations directory, regardless of where `go test` was invoked
// from. golang-migrate's file:// source needs an absolute path.
func migrationsPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// internal/store/integration_test.go → repo root → db/migrations
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	abs, err := filepath.Abs(filepath.Join(root, "db", "migrations"))
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	return abs
}

// setupPostgres boots a postgres:16 container, runs the migrations,
// and returns a Store wrapping a connected pool. The cleanup function
// terminates the container and closes the pool.
func setupPostgres(t *testing.T) (Store, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("ocs_testbench"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("postgres container start: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("ConnectionString: %v", err)
	}

	// Run migrations against the live container.
	mig, err := migrate.New("file://"+migrationsPath(t), dsn)
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate.Up: %v", err)
	}
	if srcErr, dbErr := mig.Close(); srcErr != nil || dbErr != nil {
		t.Fatalf("migrate.Close: src=%v db=%v", srcErr, dbErr)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		// Use a fresh context — the test's may have been cancelled.
		termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer termCancel()
		_ = container.Terminate(termCtx)
	})

	return NewStore(pool), pool
}

// peerBody is a representative JSONB peer config covering identity,
// connection, CER capabilities, watchdog, and startup blocks. The
// shape mirrors the design plan; the integration test uses it to
// confirm AC-2 (lossless round-trip).
const peerBody = `{
  "identity": {
    "origin_host": "ocs.example.net",
    "origin_realm": "example.net"
  },
  "connection": {
    "host": "127.0.0.1",
    "port": 3868,
    "transport": "tcp"
  },
  "cer": {
    "vendor_id": 10415,
    "product_name": "ocs-testbench",
    "supported_apps": [4]
  },
  "watchdog": {
    "interval_ms": 30000,
    "timeout_ms": 5000
  },
  "startup": {
    "send_cer_on_connect": true,
    "wait_for_cea_ms": 5000
  }
}`

// templateBody is a representative JSONB AVP template body with two
// MSCC blocks, used for AC-4.
const templateBody = `{
  "session_id": "{{generated}}",
  "service_context_id": "32251@3gpp.org",
  "mscc": [
    {
      "rating_group": 1,
      "service_identifier": 100,
      "requested_units": {"total_octets": 1048576}
    },
    {
      "rating_group": 2,
      "service_identifier": 200,
      "requested_units": {"total_octets": 524288}
    }
  ]
}`

// scenarioBody is a representative JSONB scenario step list with
// extractions, guards, and assertions, used for AC-6.
const scenarioBody = `{
  "variables": {"subscriber_msisdn": "27821234567"},
  "services": ["voice", "data"],
  "steps": [
    {
      "id": "ccr-i",
      "type": "request",
      "extractions": [{"name": "session_id", "from": "Session-Id"}],
      "guards": [{"check": "Result-Code", "equals": 2001}],
      "assertions": [{"path": "MSCC[0].Granted-Service-Unit.CC-Total-Octets", "gt": 0}]
    },
    {
      "id": "ccr-u",
      "type": "request",
      "extractions": [],
      "guards": [{"check": "Result-Code", "equals": 2001}],
      "assertions": []
    }
  ]
}`

// AC-1 — schema landed: every required table exists with the columns,
// constraints, FKs, and unique indexes the migration declares.
func TestIntegration_AC1_SchemaCreatedByMigrations(t *testing.T) {
	_, pool := setupPostgres(t)
	ctx := context.Background()

	tables := []string{"peer", "subscriber", "avp_template", "scenario", "custom_dictionary"}
	for _, tbl := range tables {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`, tbl).Scan(&exists)
		if err != nil {
			t.Fatalf("query for %s: %v", tbl, err)
		}
		if !exists {
			t.Fatalf("table %s missing after migrations", tbl)
		}
	}

	// Foreign keys on scenario.
	var fkCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.table_constraints
		WHERE table_name = 'scenario' AND constraint_type = 'FOREIGN KEY'`).Scan(&fkCount); err != nil {
		t.Fatalf("FK query: %v", err)
	}
	if fkCount != 2 {
		t.Fatalf("scenario FK count: got %d want 2", fkCount)
	}

	// Unique constraints on the four tables that declare them.
	for _, tbl := range []string{"peer", "avp_template", "scenario", "custom_dictionary"} {
		var uqCount int
		if err := pool.QueryRow(ctx, `
			SELECT count(*)
			FROM information_schema.table_constraints
			WHERE table_name = $1 AND constraint_type = 'UNIQUE'`, tbl).Scan(&uqCount); err != nil {
			t.Fatalf("unique query for %s: %v", tbl, err)
		}
		if uqCount < 1 {
			t.Fatalf("%s should declare at least one UNIQUE constraint", tbl)
		}
	}
}

// AC-2 — peer JSONB body round-trips losslessly across insert/get.
func TestIntegration_AC2_PeerJSONBRoundTrip(t *testing.T) {
	s, _ := setupPostgres(t)
	got, err := s.InsertPeer(context.Background(), "PCEF-A", []byte(peerBody))
	if err != nil {
		t.Fatalf("InsertPeer: %v", err)
	}
	fetched, err := s.GetPeer(context.Background(), got.ID)
	if err != nil {
		t.Fatalf("GetPeer: %v", err)
	}
	assertJSONEqual(t, peerBody, string(fetched.Body))
}

// AC-3 — subscriber without IMEI stores NULL for that field.
func TestIntegration_AC3_SubscriberWithoutImeiStoresNull(t *testing.T) {
	s, _ := setupPostgres(t)
	in := InsertSubscriberParams{
		Name:        "no-imei",
		Msisdn:      "27821234567",
		Iccid:       "8927000000000000001",
		Imei:        pgtype.Text{Valid: false},
		DeviceMake:  pgtype.Text{Valid: false},
		DeviceModel: pgtype.Text{Valid: false},
	}
	got, err := s.InsertSubscriber(context.Background(), in)
	if err != nil {
		t.Fatalf("InsertSubscriber: %v", err)
	}
	fetched, err := s.GetSubscriber(context.Background(), got.ID)
	if err != nil {
		t.Fatalf("GetSubscriber: %v", err)
	}
	if fetched.Imei.Valid {
		t.Fatalf("Imei should be NULL; got %+v", fetched.Imei)
	}
	if fetched.DeviceMake.Valid || fetched.DeviceModel.Valid {
		t.Fatalf("device fields should be NULL; got %+v %+v", fetched.DeviceMake, fetched.DeviceModel)
	}
}

// AC-4 — AVP template JSONB body with MSCC blocks round-trips.
func TestIntegration_AC4_AVPTemplateJSONBRoundTrip(t *testing.T) {
	s, _ := setupPostgres(t)
	got, err := s.InsertAVPTemplate(context.Background(), "MSCC-Template", []byte(templateBody))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	fetched, err := s.GetAVPTemplate(context.Background(), got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertJSONEqual(t, templateBody, string(fetched.Body))
}

// AC-5 — scenario insert with non-existent template_id or peer_id
// fails with a Postgres FK error, surfaced as ErrForeignKey.
func TestIntegration_AC5_ScenarioInsertWithMissingFK_FailsWithForeignKey(t *testing.T) {
	s, _ := setupPostgres(t)

	// First insert a real peer and template — needed so we can isolate
	// the FK violation to one side at a time.
	peer, err := s.InsertPeer(context.Background(), "p", []byte(`{}`))
	if err != nil {
		t.Fatalf("InsertPeer: %v", err)
	}
	tpl, err := s.InsertAVPTemplate(context.Background(), "t", []byte(`{}`))
	if err != nil {
		t.Fatalf("InsertAVPTemplate: %v", err)
	}

	var ghost pgtype.UUID
	ghost.Bytes = [16]byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x00, 0x40, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	ghost.Valid = true

	// Ghost template_id.
	if _, err := s.InsertScenario(context.Background(), "fk-1", ghost, peer.ID, []byte(`{}`)); !errors.Is(err, ErrForeignKey) {
		t.Fatalf("ghost template_id: got %v want ErrForeignKey", err)
	}
	// Ghost peer_id.
	if _, err := s.InsertScenario(context.Background(), "fk-2", tpl.ID, ghost, []byte(`{}`)); !errors.Is(err, ErrForeignKey) {
		t.Fatalf("ghost peer_id: got %v want ErrForeignKey", err)
	}
}

// AC-6 — scenario JSONB body with extractions, guards, and assertions
// round-trips losslessly.
func TestIntegration_AC6_ScenarioJSONBRoundTrip(t *testing.T) {
	s, _ := setupPostgres(t)
	peer, _ := s.InsertPeer(context.Background(), "p", []byte(`{}`))
	tpl, _ := s.InsertAVPTemplate(context.Background(), "t", []byte(`{}`))
	got, err := s.InsertScenario(context.Background(), "sc", tpl.ID, peer.ID, []byte(scenarioBody))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	fetched, err := s.GetScenario(context.Background(), got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertJSONEqual(t, scenarioBody, string(fetched.Body))
}

// AC-7 — duplicate peer name fails with unique constraint error
// surfaced as ErrDuplicateName.
func TestIntegration_AC7_DuplicatePeerName_FailsWithDuplicate(t *testing.T) {
	s, _ := setupPostgres(t)
	if _, err := s.InsertPeer(context.Background(), "PCEF-A", []byte(`{}`)); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := s.InsertPeer(context.Background(), "PCEF-A", []byte(`{}`))
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("err: got %v want ErrDuplicateName", err)
	}
}

// AC-8 — custom dictionary round-trips name, description, full XML
// content, and active flag.
func TestIntegration_AC8_CustomDictionaryRoundTrip(t *testing.T) {
	s, _ := setupPostgres(t)
	xml := `<?xml version="1.0"?>
<dictionary>
  <vendor id="10415" name="3GPP"/>
  <avp name="Custom-Foo" code="65000" type="UTF8String"/>
</dictionary>`
	got, err := s.InsertCustomDictionary(context.Background(), "VendorX",
		pgtype.Text{String: "Vendor X custom AVPs", Valid: true},
		xml, true)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	fetched, err := s.GetCustomDictionary(context.Background(), got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.Name != "VendorX" {
		t.Fatalf("Name: got %q", fetched.Name)
	}
	if fetched.Description.String != "Vendor X custom AVPs" {
		t.Fatalf("Description: got %q", fetched.Description.String)
	}
	if fetched.XmlContent != xml {
		t.Fatalf("xml mismatch — content lost in round-trip")
	}
	if !fetched.IsActive {
		t.Fatalf("IsActive should be true")
	}
}

// AC-9 — covered by the unit tests in store_test.go; this guard
// confirms NewTestStore satisfies the same Store interface so
// callers can swap implementations without changing test code.
func TestIntegration_AC9_TestStoreSatisfiesStoreInterface(t *testing.T) {
	var _ Store = NewTestStore()
}

// assertJSONEqual normalises two JSON strings (decode + re-encode)
// and compares the canonical forms. Postgres may reformat JSONB on
// the way out (key ordering, whitespace), so a byte-for-byte
// comparison is too strict for the round-trip assertion.
func assertJSONEqual(t *testing.T, want, got string) {
	t.Helper()
	var wantV, gotV interface{}
	if err := json.Unmarshal([]byte(want), &wantV); err != nil {
		t.Fatalf("decode want: %v", err)
	}
	if err := json.Unmarshal([]byte(got), &gotV); err != nil {
		t.Fatalf("decode got: %v", err)
	}
	wantBytes, _ := json.Marshal(wantV)
	gotBytes, _ := json.Marshal(gotV)
	if string(wantBytes) != string(gotBytes) {
		t.Fatalf("JSON mismatch\n  want: %s\n  got:  %s", wantBytes, gotBytes)
	}
}
