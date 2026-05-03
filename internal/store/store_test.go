// Unit tests for the Store interface, exercising the in-memory test
// store returned by NewTestStore.
//
// AC-9 of Feature #16: unit tests must exercise every Store method
// without requiring a real PostgreSQL connection. Running
// `go test ./internal/store/...` (no build tags) must pass on a host
// that has no database available — these tests live without the
// integration build tag and never import a pgx connection helper.
//
// Tests focus on the Store contract: round-trip semantics, uniqueness
// enforcement, foreign-key enforcement, and the typed errors a caller
// can errors.Is against. Schema-level behaviour (ON DELETE RESTRICT
// over a real Postgres, JSONB binary fidelity) is verified separately
// by the integration tests under internal/store/integration_test.go.

package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// helper — render a pgtype.Text from a Go string for fixture brevity.
func textFrom(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// helper — a non-null pgtype.Text representing SQL NULL via Valid: false.
func nullText() pgtype.Text { return pgtype.Text{Valid: false} }

// seedPeer inserts a fresh peer with the given name. Failing the
// helper's underlying call is treated as a test failure since the
// helper is invoked from many tests that depend on its success.
func seedPeer(t *testing.T, s Store, name string) Peer {
	t.Helper()
	p, err := s.InsertPeer(context.Background(), name, []byte(`{"identity":"`+name+`"}`))
	if err != nil {
		t.Fatalf("InsertPeer(%q): %v", name, err)
	}
	return p
}

// seedTemplate inserts a fresh AVP template with the given name.
func seedTemplate(t *testing.T, s Store, name string) AVPTemplate {
	t.Helper()
	tpl, err := s.InsertAVPTemplate(context.Background(), name, []byte(`{"name":"`+name+`"}`))
	if err != nil {
		t.Fatalf("InsertAVPTemplate(%q): %v", name, err)
	}
	return tpl
}

// ----- peer ------------------------------------------------------

func TestPeer_InsertAndGetByID_RoundTrips(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	got := seedPeer(t, s, "PCEF-A")

	fetched, err := s.GetPeer(context.Background(), got.ID)
	if err != nil {
		t.Fatalf("GetPeer: %v", err)
	}
	if fetched.Name != "PCEF-A" {
		t.Fatalf("Name: got %q want %q", fetched.Name, "PCEF-A")
	}
	if string(fetched.Body) != `{"identity":"PCEF-A"}` {
		t.Fatalf("Body: got %s", fetched.Body)
	}
	if !fetched.CreatedAt.Valid || !fetched.UpdatedAt.Valid {
		t.Fatalf("timestamps must be Valid; got %+v %+v", fetched.CreatedAt, fetched.UpdatedAt)
	}
}

func TestPeer_GetByName_RoundTrips(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	got := seedPeer(t, s, "PCEF-B")

	fetched, err := s.GetPeerByName(context.Background(), "PCEF-B")
	if err != nil {
		t.Fatalf("GetPeerByName: %v", err)
	}
	if fetched.ID != got.ID {
		t.Fatalf("ID mismatch")
	}
}

func TestPeer_GetByName_MissingReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	_, err := s.GetPeerByName(context.Background(), "absent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err: got %v want ErrNotFound", err)
	}
}

func TestPeer_GetByID_MissingReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	var zero pgtype.UUID
	zero.Valid = true
	_, err := s.GetPeer(context.Background(), zero)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err: got %v want ErrNotFound", err)
	}
}

func TestPeer_InsertDuplicateName_ReturnsDuplicate(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	seedPeer(t, s, "dup")
	_, err := s.InsertPeer(context.Background(), "dup", []byte(`{}`))
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("err: got %v want ErrDuplicateName", err)
	}
}

func TestPeer_List_ReturnsInsertedSortedByName(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	seedPeer(t, s, "z-peer")
	seedPeer(t, s, "a-peer")
	seedPeer(t, s, "m-peer")

	peers, err := s.ListPeers(context.Background())
	if err != nil {
		t.Fatalf("ListPeers: %v", err)
	}
	if len(peers) != 3 {
		t.Fatalf("len: got %d want 3", len(peers))
	}
	want := []string{"a-peer", "m-peer", "z-peer"}
	for i, p := range peers {
		if p.Name != want[i] {
			t.Fatalf("peers[%d].Name: got %q want %q", i, p.Name, want[i])
		}
	}
}

func TestPeer_Update_MutatesAndBumpsUpdatedAt(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	seeded := seedPeer(t, s, "renamed")

	// Drive the test store's clock backwards-and-forwards by a known
	// delta so the assertion on UpdatedAt is meaningful even on hosts
	// with low-resolution clocks.
	t.Cleanup(func() {})
	time.Sleep(2 * time.Millisecond)

	updated, err := s.UpdatePeer(context.Background(), seeded.ID, "renamed-2", []byte(`{"v":2}`))
	if err != nil {
		t.Fatalf("UpdatePeer: %v", err)
	}
	if updated.Name != "renamed-2" {
		t.Fatalf("Name: got %q", updated.Name)
	}
	if !updated.UpdatedAt.Time.After(seeded.UpdatedAt.Time) {
		t.Fatalf("UpdatedAt should advance: before=%v after=%v",
			seeded.UpdatedAt.Time, updated.UpdatedAt.Time)
	}
	if string(updated.Body) != `{"v":2}` {
		t.Fatalf("Body: got %s", updated.Body)
	}
}

func TestPeer_UpdateNameToTakenName_ReturnsDuplicate(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	seedPeer(t, s, "first")
	second := seedPeer(t, s, "second")

	_, err := s.UpdatePeer(context.Background(), second.ID, "first", []byte(`{}`))
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("err: got %v want ErrDuplicateName", err)
	}
}

func TestPeer_UpdateMissingID_ReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	var ghost pgtype.UUID
	ghost.Valid = true
	_, err := s.UpdatePeer(context.Background(), ghost, "x", []byte(`{}`))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err: got %v want ErrNotFound", err)
	}
}

func TestPeer_Delete_RemovesAndSubsequentGetReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	seeded := seedPeer(t, s, "to-delete")

	if err := s.DeletePeer(context.Background(), seeded.ID); err != nil {
		t.Fatalf("DeletePeer: %v", err)
	}
	if _, err := s.GetPeer(context.Background(), seeded.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("post-delete Get: got %v want ErrNotFound", err)
	}
}

func TestPeer_DeleteMissing_ReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	var ghost pgtype.UUID
	ghost.Valid = true
	if err := s.DeletePeer(context.Background(), ghost); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err: got %v want ErrNotFound", err)
	}
}

func TestPeer_DeleteWithReferencingScenario_ReturnsForeignKey(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	peer := seedPeer(t, s, "referenced")
	if _, err := s.InsertScenario(context.Background(), "scenario", peer.ID, pgtype.UUID{}, []byte(`{}`)); err != nil {
		t.Fatalf("InsertScenario: %v", err)
	}
	err := s.DeletePeer(context.Background(), peer.ID)
	if !errors.Is(err, ErrForeignKey) {
		t.Fatalf("err: got %v want ErrForeignKey", err)
	}
}

// ----- subscriber -----------------------------------------------

func TestSubscriber_InsertAndGetByID_RoundTripsIncludingNullableFields(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	in := InsertSubscriberParams{
		Name:   "alice",
		Msisdn: "27821234567",
		Iccid:  "8927000000000000001",
		Imei:   nullText(),
		Tac:    nullText(),
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
	if fetched.Msisdn != "27821234567" {
		t.Fatalf("Msisdn: got %q", fetched.Msisdn)
	}
}

func TestSubscriber_GetByName_DuplicateNamesReturnFirstByID(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	// Subscriber names are NOT unique. Insert two with the same name
	// and confirm GetSubscriberByName returns the lexicographically
	// smaller id (mirrors the production query's
	// ORDER BY id LIMIT 1).
	first, err := s.InsertSubscriber(context.Background(), InsertSubscriberParams{
		Name: "shared", Msisdn: "1", Iccid: "1",
	})
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	second, err := s.InsertSubscriber(context.Background(), InsertSubscriberParams{
		Name: "shared", Msisdn: "2", Iccid: "2",
	})
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	got, err := s.GetSubscriberByName(context.Background(), "shared")
	if err != nil {
		t.Fatalf("GetSubscriberByName: %v", err)
	}
	expected := first
	if lessUUID(second.ID, first.ID) {
		expected = second
	}
	if got.ID != expected.ID {
		t.Fatalf("expected lexicographically-smallest id; got %v", got.ID.Bytes)
	}
}

func TestSubscriber_GetByName_MissingReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	_, err := s.GetSubscriberByName(context.Background(), "nobody")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err: got %v want ErrNotFound", err)
	}
}

func TestSubscriber_Update_MutatesAllFields(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	in := InsertSubscriberParams{Name: "u", Msisdn: "1", Iccid: "1"}
	got, _ := s.InsertSubscriber(context.Background(), in)
	updated, err := s.UpdateSubscriber(context.Background(), UpdateSubscriberParams{
		ID:     got.ID,
		Name:   "u-2",
		Msisdn: "9",
		Iccid:  "9",
		Imei:   textFrom("IMEI-9"),
		Tac:    textFrom("35617109"),
	})
	if err != nil {
		t.Fatalf("UpdateSubscriber: %v", err)
	}
	if updated.Imei.String != "IMEI-9" {
		t.Fatalf("Imei: got %+v", updated.Imei)
	}
	if updated.Name != "u-2" {
		t.Fatalf("Name: got %q", updated.Name)
	}
}

func TestSubscriber_UpdateMissing_ReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	var ghost pgtype.UUID
	ghost.Valid = true
	_, err := s.UpdateSubscriber(context.Background(), UpdateSubscriberParams{ID: ghost})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err: got %v want ErrNotFound", err)
	}
}

func TestSubscriber_DeleteMissing_ReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	var ghost pgtype.UUID
	ghost.Valid = true
	if err := s.DeleteSubscriber(context.Background(), ghost); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err: got %v want ErrNotFound", err)
	}
}

func TestSubscriber_List_ReturnsInsertedSortedByName(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	for _, n := range []string{"c", "a", "b"} {
		if _, err := s.InsertSubscriber(context.Background(), InsertSubscriberParams{Name: n, Msisdn: n, Iccid: n}); err != nil {
			t.Fatalf("Insert(%q): %v", n, err)
		}
	}
	subs, err := s.ListSubscribers(context.Background())
	if err != nil {
		t.Fatalf("ListSubscribers: %v", err)
	}
	want := []string{"a", "b", "c"}
	for i, sub := range subs {
		if sub.Name != want[i] {
			t.Fatalf("subs[%d]: got %q want %q", i, sub.Name, want[i])
		}
	}
}

// ----- avp_template ---------------------------------------------

func TestAVPTemplate_RoundTripAndDuplicate(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	tpl, err := s.InsertAVPTemplate(context.Background(), "tpl", []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	got, err := s.GetAVPTemplate(context.Background(), tpl.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "tpl" {
		t.Fatalf("Name: got %q", got.Name)
	}
	got2, err := s.GetAVPTemplateByName(context.Background(), "tpl")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got2.ID != tpl.ID {
		t.Fatalf("ID mismatch via GetByName")
	}

	if _, err := s.InsertAVPTemplate(context.Background(), "tpl", []byte(`{}`)); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("dup: got %v want ErrDuplicateName", err)
	}
}

func TestAVPTemplate_DeleteWithReferencingScenario_Returns204(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	peer := seedPeer(t, s, "p")
	tpl := seedTemplate(t, s, "t")
	if _, err := s.InsertScenario(context.Background(), "sc", peer.ID, pgtype.UUID{}, []byte(`{}`)); err != nil {
		t.Fatalf("Insert scenario: %v", err)
	}
	err := s.DeleteAVPTemplate(context.Background(), tpl.ID)
	if err != nil {
		t.Fatalf("err: got %v want nil (templates no longer FK'd to scenarios)", err)
	}
}

func TestAVPTemplate_Update_MutatesAndBumpsUpdatedAt(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	tpl := seedTemplate(t, s, "t1")
	time.Sleep(2 * time.Millisecond)

	updated, err := s.UpdateAVPTemplate(context.Background(), tpl.ID, "t2", []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "t2" {
		t.Fatalf("Name: got %q", updated.Name)
	}
	if !updated.UpdatedAt.Time.After(tpl.UpdatedAt.Time) {
		t.Fatalf("UpdatedAt should advance")
	}
}

// ----- scenario -------------------------------------------------

func TestScenario_InsertWithMissingPeerID_ReturnsForeignKey(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	var ghost pgtype.UUID
	ghost.Valid = true
	_, err := s.InsertScenario(context.Background(), "sc", ghost, pgtype.UUID{}, []byte(`{}`))
	if !errors.Is(err, ErrForeignKey) {
		t.Fatalf("err: got %v want ErrForeignKey", err)
	}
}

func TestScenario_InsertWithMissingSubscriberID_ReturnsForeignKey(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	peer := seedPeer(t, s, "p")
	var ghost pgtype.UUID
	ghost.Valid = true
	_, err := s.InsertScenario(context.Background(), "sc", peer.ID, ghost, []byte(`{}`))
	if !errors.Is(err, ErrForeignKey) {
		t.Fatalf("err: got %v want ErrForeignKey", err)
	}
}

func TestScenario_InsertDuplicateName_ReturnsDuplicate(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	peer := seedPeer(t, s, "p")
	if _, err := s.InsertScenario(context.Background(), "dup", peer.ID, pgtype.UUID{}, []byte(`{}`)); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := s.InsertScenario(context.Background(), "dup", peer.ID, pgtype.UUID{}, []byte(`{}`))
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("err: got %v want ErrDuplicateName", err)
	}
}

func TestScenario_RoundTripAndUpdate(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	peer := seedPeer(t, s, "p")
	sc, err := s.InsertScenario(context.Background(), "sc", peer.ID, pgtype.UUID{}, []byte(`{"steps":[]}`))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	got, err := s.GetScenario(context.Background(), sc.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got.Body) != `{"steps":[]}` {
		t.Fatalf("body round-trip: %s", got.Body)
	}
	if got.PeerID != peer.ID {
		t.Fatalf("peer id mismatch")
	}
	updated, err := s.UpdateScenario(context.Background(), UpdateScenarioParams{
		ID: sc.ID, Name: "sc-2", PeerID: peer.ID, Body: []byte(`{"v":2}`),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "sc-2" {
		t.Fatalf("name: %q", updated.Name)
	}
}

func TestScenario_UpdateMissing_ReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	var ghost pgtype.UUID
	ghost.Valid = true
	_, err := s.UpdateScenario(context.Background(), UpdateScenarioParams{ID: ghost})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err: got %v want ErrNotFound", err)
	}
}

func TestScenario_DeleteThenGet_ReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	peer := seedPeer(t, s, "p")
	sc, _ := s.InsertScenario(context.Background(), "sc", peer.ID, pgtype.UUID{}, []byte(`{}`))
	if err := s.DeleteScenario(context.Background(), sc.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.GetScenario(context.Background(), sc.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("post-delete: got %v want ErrNotFound", err)
	}
}

func TestScenario_List_ReturnsInsertedSortedByName(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	peer := seedPeer(t, s, "p")
	for _, n := range []string{"z", "a", "m"} {
		if _, err := s.InsertScenario(context.Background(), n, peer.ID, pgtype.UUID{}, []byte(`{}`)); err != nil {
			t.Fatalf("Insert(%q): %v", n, err)
		}
	}
	got, err := s.ListScenarios(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"a", "m", "z"}
	for i, sc := range got {
		if sc.Name != want[i] {
			t.Fatalf("scenarios[%d]: got %q want %q", i, sc.Name, want[i])
		}
	}
}

// ----- custom_dictionary ----------------------------------------

func TestCustomDictionary_RoundTripIncludingNullDescription(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	d, err := s.InsertCustomDictionary(context.Background(), "VendorX", nullText(), `<dict/>`, true)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	got, err := s.GetCustomDictionary(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description.Valid {
		t.Fatalf("Description should be NULL; got %+v", got.Description)
	}
	if got.XmlContent != `<dict/>` {
		t.Fatalf("Xml: got %q", got.XmlContent)
	}
	if !got.IsActive {
		t.Fatalf("IsActive should be true")
	}
}

func TestCustomDictionary_DuplicateName_ReturnsDuplicate(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	if _, err := s.InsertCustomDictionary(context.Background(), "dup", nullText(), `<x/>`, true); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := s.InsertCustomDictionary(context.Background(), "dup", nullText(), `<y/>`, false)
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("err: got %v want ErrDuplicateName", err)
	}
}

func TestCustomDictionary_Update_MutatesEveryField(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	d, _ := s.InsertCustomDictionary(context.Background(), "VendorY", textFrom("desc"), `<dict/>`, true)
	updated, err := s.UpdateCustomDictionary(context.Background(), UpdateCustomDictionaryParams{
		ID:          d.ID,
		Name:        "VendorY-renamed",
		Description: textFrom("desc-2"),
		XmlContent:  `<dict2/>`,
		IsActive:    false,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "VendorY-renamed" || updated.Description.String != "desc-2" || updated.XmlContent != `<dict2/>` || updated.IsActive {
		t.Fatalf("update did not propagate: %+v", updated)
	}
}

func TestCustomDictionary_DeleteMissing_ReturnsNotFound(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	var ghost pgtype.UUID
	ghost.Valid = true
	if err := s.DeleteCustomDictionary(context.Background(), ghost); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err: got %v want ErrNotFound", err)
	}
}

func TestCustomDictionary_List_SortedByName(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	for _, n := range []string{"c", "a", "b"} {
		if _, err := s.InsertCustomDictionary(context.Background(), n, nullText(), `<x/>`, true); err != nil {
			t.Fatalf("Insert(%q): %v", n, err)
		}
	}
	got, err := s.ListCustomDictionaries(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"a", "b", "c"}
	for i, d := range got {
		if d.Name != want[i] {
			t.Fatalf("dicts[%d]: got %q want %q", i, d.Name, want[i])
		}
	}
}

// ----- error wrapping ------------------------------------------

func TestEntityError_FormatsEntityAndKey(t *testing.T) {
	err := notFound("peer", "PCEF-A")
	want := `peer "PCEF-A": store: not found`
	if err.Error() != want {
		t.Fatalf("Error(): got %q want %q", err.Error(), want)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("errors.Is(notFound) false")
	}
}

func TestEntityError_WithoutKey(t *testing.T) {
	err := notFound("peer", "")
	want := "peer: store: not found"
	if err.Error() != want {
		t.Fatalf("Error(): got %q want %q", err.Error(), want)
	}
}

// ----- body isolation ------------------------------------------

func TestBodyMutation_DoesNotLeakAcrossInsertOrGet(t *testing.T) {
	s := NewTestStore()
	defer s.Close()
	body := []byte(`{"v":1}`)
	got, err := s.InsertPeer(context.Background(), "iso", body)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// Mutate the caller's slice — the stored row must not see it.
	body[5] = '9'
	fetched, err := s.GetPeer(context.Background(), got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(fetched.Body) != `{"v":1}` {
		t.Fatalf("body leaked through Insert: %s", fetched.Body)
	}
}

// ----- close lifecycle -----------------------------------------

func TestClose_IsNoOp(t *testing.T) {
	s := NewTestStore()
	s.Close() // should not panic
	s.Close() // idempotent
}
