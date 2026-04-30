// In-memory Store implementation for unit tests.
//
// The test store keeps each entity in a map keyed by its uuid bytes,
// plus a secondary name → id index for the four tables that declare a
// unique-name constraint (peer, avp_template, scenario,
// custom_dictionary). Mutations enforce the same uniqueness and
// foreign-key invariants the schema enforces, so tests written
// against the test store assert on the same error sentinels they
// would see in production.
//
// AC-9 of Feature #16 hinges on this: unit tests must exercise the
// Store contract without a live database. The test store is
// minimal-but-real — actual round-trips through maps — rather than
// a generated mock, so the interface contract gets early verification
// every time a developer runs `go test ./...`.

package store

import (
	"context"
	"crypto/rand"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// NewTestStore returns an in-memory Store. It accepts no arguments
// because tests construct fixtures by calling Insert* directly on
// the returned interface; there is no backing pool to configure.
//
// The returned store is goroutine-safe — every method takes a single
// mutex — so concurrent tests (or t.Parallel) can share one store
// without race-condition surprises.
func NewTestStore() Store {
	return &testStore{
		peers:      map[[16]byte]Peer{},
		peerByName: map[string][16]byte{},
		subs:       map[[16]byte]Subscriber{},
		templates:  map[[16]byte]AVPTemplate{},
		tplByName:  map[string][16]byte{},
		scenarios:  map[[16]byte]Scenario{},
		scenByName: map[string][16]byte{},
		dicts:      map[[16]byte]CustomDictionary{},
		dictByName: map[string][16]byte{},
		now:        func() time.Time { return time.Now().UTC() },
		newID:      randomUUID,
	}
}

// testStore is the in-memory implementation. Every method takes the
// same mutex; the store is not high-throughput by design.
type testStore struct {
	mu sync.Mutex

	peers      map[[16]byte]Peer
	peerByName map[string][16]byte

	// Subscribers do not have a unique-name constraint, so the
	// secondary index is not maintained. GetSubscriberByName scans
	// the map to find the lexicographically-first matching id, which
	// matches the production query (LIMIT 1 with ORDER BY id).
	subs map[[16]byte]Subscriber

	templates map[[16]byte]AVPTemplate
	tplByName map[string][16]byte

	scenarios  map[[16]byte]Scenario
	scenByName map[string][16]byte

	dicts      map[[16]byte]CustomDictionary
	dictByName map[string][16]byte

	// now and newID are injected so tests that need deterministic
	// timestamps or ids can swap them without touching package-level
	// globals.
	now   func() time.Time
	newID func() pgtype.UUID
}

// Close is a no-op — the test store has no resources to release.
func (t *testStore) Close() {}

// randomUUID generates a v4 UUID and returns it as a pgtype.UUID
// value. The wrapper exists so the Store interface and the test
// store agree on the UUID shape (pgtype.UUID, not google/uuid.UUID,
// to avoid a dependency this package does not otherwise need).
func randomUUID() pgtype.UUID {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read is documented to never fail on linux;
		// surfacing the error inside a test helper is more noise
		// than signal. Panic so a broken environment is loud.
		panic("store: crypto/rand failed: " + err.Error())
	}
	// Set version (4) and variant (RFC 4122) bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return pgtype.UUID{Bytes: b, Valid: true}
}

// timestamptz wraps a time.Time as a non-null pgtype.Timestamptz so
// the test store's row models match the sqlc-generated shape.
func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// ----- peer ------------------------------------------------------

func (t *testStore) InsertPeer(ctx context.Context, name string, body []byte) (Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, taken := t.peerByName[name]; taken {
		return Peer{}, duplicate("peer", name)
	}
	id := t.newID()
	now := timestamptz(t.now())
	row := Peer{
		ID:        id,
		Name:      name,
		Body:      cloneBytes(body),
		CreatedAt: now,
		UpdatedAt: now,
	}
	t.peers[uuidKey(id)] = row
	t.peerByName[name] = uuidKey(id)
	return row, nil
}

func (t *testStore) GetPeer(ctx context.Context, id pgtype.UUID) (Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.peers[uuidKey(id)]
	if !ok {
		return Peer{}, notFound("peer", "")
	}
	return row, nil
}

func (t *testStore) GetPeerByName(ctx context.Context, name string) (Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key, ok := t.peerByName[name]
	if !ok {
		return Peer{}, notFound("peer", name)
	}
	return t.peers[key], nil
}

func (t *testStore) ListPeers(ctx context.Context) ([]Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Peer, 0, len(t.peers))
	for _, p := range t.peers {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (t *testStore) UpdatePeer(ctx context.Context, id pgtype.UUID, name string, body []byte) (Peer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	row, ok := t.peers[uuidKey(id)]
	if !ok {
		return Peer{}, notFound("peer", "")
	}
	// Reject name collisions unless the caller is renaming a row to
	// the name it already has.
	if other, taken := t.peerByName[name]; taken && other != uuidKey(id) {
		return Peer{}, duplicate("peer", name)
	}
	delete(t.peerByName, row.Name)
	row.Name = name
	row.Body = cloneBytes(body)
	row.UpdatedAt = timestamptz(t.now())
	t.peers[uuidKey(id)] = row
	t.peerByName[name] = uuidKey(id)
	return row, nil
}

func (t *testStore) DeletePeer(ctx context.Context, id pgtype.UUID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.peers[uuidKey(id)]
	if !ok {
		return notFound("peer", "")
	}
	// Schema's ON DELETE RESTRICT — cannot drop a peer that any
	// scenario still references.
	for _, sc := range t.scenarios {
		if sc.PeerID == id {
			return foreignKey("peer", row.Name)
		}
	}
	delete(t.peers, uuidKey(id))
	delete(t.peerByName, row.Name)
	return nil
}

// ----- subscriber -----------------------------------------------

func (t *testStore) InsertSubscriber(ctx context.Context, arg InsertSubscriberParams) (Subscriber, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := t.newID()
	now := timestamptz(t.now())
	row := Subscriber{
		ID:          id,
		Name:        arg.Name,
		Msisdn:      arg.Msisdn,
		Iccid:       arg.Iccid,
		Imei:        arg.Imei,
		DeviceMake:  arg.DeviceMake,
		DeviceModel: arg.DeviceModel,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	t.subs[uuidKey(id)] = row
	return row, nil
}

func (t *testStore) GetSubscriber(ctx context.Context, id pgtype.UUID) (Subscriber, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.subs[uuidKey(id)]
	if !ok {
		return Subscriber{}, notFound("subscriber", "")
	}
	return row, nil
}

func (t *testStore) GetSubscriberByName(ctx context.Context, name string) (Subscriber, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Subscriber names are not unique. Find the row with the smallest
	// id that has the requested name, mirroring the production query
	// (ORDER BY id LIMIT 1).
	var best Subscriber
	found := false
	for _, row := range t.subs {
		if row.Name != name {
			continue
		}
		if !found || lessUUID(row.ID, best.ID) {
			best = row
			found = true
		}
	}
	if !found {
		return Subscriber{}, notFound("subscriber", name)
	}
	return best, nil
}

func (t *testStore) ListSubscribers(ctx context.Context) ([]Subscriber, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Subscriber, 0, len(t.subs))
	for _, s := range t.subs {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (t *testStore) UpdateSubscriber(ctx context.Context, arg UpdateSubscriberParams) (Subscriber, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.subs[uuidKey(arg.ID)]
	if !ok {
		return Subscriber{}, notFound("subscriber", "")
	}
	row.Name = arg.Name
	row.Msisdn = arg.Msisdn
	row.Iccid = arg.Iccid
	row.Imei = arg.Imei
	row.DeviceMake = arg.DeviceMake
	row.DeviceModel = arg.DeviceModel
	row.UpdatedAt = timestamptz(t.now())
	t.subs[uuidKey(arg.ID)] = row
	return row, nil
}

func (t *testStore) DeleteSubscriber(ctx context.Context, id pgtype.UUID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.subs[uuidKey(id)]; !ok {
		return notFound("subscriber", "")
	}
	delete(t.subs, uuidKey(id))
	return nil
}

// ----- avp_template ---------------------------------------------

func (t *testStore) InsertAVPTemplate(ctx context.Context, name string, body []byte) (AVPTemplate, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, taken := t.tplByName[name]; taken {
		return AVPTemplate{}, duplicate("avp_template", name)
	}
	id := t.newID()
	now := timestamptz(t.now())
	row := AVPTemplate{
		ID:        id,
		Name:      name,
		Body:      cloneBytes(body),
		CreatedAt: now,
		UpdatedAt: now,
	}
	t.templates[uuidKey(id)] = row
	t.tplByName[name] = uuidKey(id)
	return row, nil
}

func (t *testStore) GetAVPTemplate(ctx context.Context, id pgtype.UUID) (AVPTemplate, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.templates[uuidKey(id)]
	if !ok {
		return AVPTemplate{}, notFound("avp_template", "")
	}
	return row, nil
}

func (t *testStore) GetAVPTemplateByName(ctx context.Context, name string) (AVPTemplate, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key, ok := t.tplByName[name]
	if !ok {
		return AVPTemplate{}, notFound("avp_template", name)
	}
	return t.templates[key], nil
}

func (t *testStore) ListAVPTemplates(ctx context.Context) ([]AVPTemplate, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]AVPTemplate, 0, len(t.templates))
	for _, x := range t.templates {
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (t *testStore) UpdateAVPTemplate(ctx context.Context, id pgtype.UUID, name string, body []byte) (AVPTemplate, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.templates[uuidKey(id)]
	if !ok {
		return AVPTemplate{}, notFound("avp_template", "")
	}
	if other, taken := t.tplByName[name]; taken && other != uuidKey(id) {
		return AVPTemplate{}, duplicate("avp_template", name)
	}
	delete(t.tplByName, row.Name)
	row.Name = name
	row.Body = cloneBytes(body)
	row.UpdatedAt = timestamptz(t.now())
	t.templates[uuidKey(id)] = row
	t.tplByName[name] = uuidKey(id)
	return row, nil
}

func (t *testStore) DeleteAVPTemplate(ctx context.Context, id pgtype.UUID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.templates[uuidKey(id)]
	if !ok {
		return notFound("avp_template", "")
	}
	for _, sc := range t.scenarios {
		if sc.TemplateID == id {
			return foreignKey("avp_template", row.Name)
		}
	}
	delete(t.templates, uuidKey(id))
	delete(t.tplByName, row.Name)
	return nil
}

// ----- scenario -------------------------------------------------

func (t *testStore) InsertScenario(ctx context.Context, name string, templateID, peerID pgtype.UUID, body []byte) (Scenario, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, taken := t.scenByName[name]; taken {
		return Scenario{}, duplicate("scenario", name)
	}
	if _, ok := t.templates[uuidKey(templateID)]; !ok {
		return Scenario{}, foreignKey("scenario", "template_id")
	}
	if _, ok := t.peers[uuidKey(peerID)]; !ok {
		return Scenario{}, foreignKey("scenario", "peer_id")
	}
	id := t.newID()
	now := timestamptz(t.now())
	row := Scenario{
		ID:         id,
		Name:       name,
		TemplateID: templateID,
		PeerID:     peerID,
		Body:       cloneBytes(body),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	t.scenarios[uuidKey(id)] = row
	t.scenByName[name] = uuidKey(id)
	return row, nil
}

func (t *testStore) GetScenario(ctx context.Context, id pgtype.UUID) (Scenario, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.scenarios[uuidKey(id)]
	if !ok {
		return Scenario{}, notFound("scenario", "")
	}
	return row, nil
}

func (t *testStore) GetScenarioByName(ctx context.Context, name string) (Scenario, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key, ok := t.scenByName[name]
	if !ok {
		return Scenario{}, notFound("scenario", name)
	}
	return t.scenarios[key], nil
}

func (t *testStore) ListScenarios(ctx context.Context) ([]Scenario, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Scenario, 0, len(t.scenarios))
	for _, s := range t.scenarios {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (t *testStore) UpdateScenario(ctx context.Context, arg UpdateScenarioParams) (Scenario, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.scenarios[uuidKey(arg.ID)]
	if !ok {
		return Scenario{}, notFound("scenario", "")
	}
	if other, taken := t.scenByName[arg.Name]; taken && other != uuidKey(arg.ID) {
		return Scenario{}, duplicate("scenario", arg.Name)
	}
	if _, ok := t.templates[uuidKey(arg.TemplateID)]; !ok {
		return Scenario{}, foreignKey("scenario", "template_id")
	}
	if _, ok := t.peers[uuidKey(arg.PeerID)]; !ok {
		return Scenario{}, foreignKey("scenario", "peer_id")
	}
	delete(t.scenByName, row.Name)
	row.Name = arg.Name
	row.TemplateID = arg.TemplateID
	row.PeerID = arg.PeerID
	row.Body = cloneBytes(arg.Body)
	row.UpdatedAt = timestamptz(t.now())
	t.scenarios[uuidKey(arg.ID)] = row
	t.scenByName[arg.Name] = uuidKey(arg.ID)
	return row, nil
}

func (t *testStore) DeleteScenario(ctx context.Context, id pgtype.UUID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.scenarios[uuidKey(id)]
	if !ok {
		return notFound("scenario", "")
	}
	delete(t.scenarios, uuidKey(id))
	delete(t.scenByName, row.Name)
	return nil
}

// ----- custom_dictionary ----------------------------------------

func (t *testStore) InsertCustomDictionary(ctx context.Context, name string, description pgtype.Text, xmlContent string, isActive bool) (CustomDictionary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, taken := t.dictByName[name]; taken {
		return CustomDictionary{}, duplicate("custom_dictionary", name)
	}
	id := t.newID()
	now := timestamptz(t.now())
	row := CustomDictionary{
		ID:          id,
		Name:        name,
		Description: description,
		XmlContent:  xmlContent,
		IsActive:    isActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	t.dicts[uuidKey(id)] = row
	t.dictByName[name] = uuidKey(id)
	return row, nil
}

func (t *testStore) GetCustomDictionary(ctx context.Context, id pgtype.UUID) (CustomDictionary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.dicts[uuidKey(id)]
	if !ok {
		return CustomDictionary{}, notFound("custom_dictionary", "")
	}
	return row, nil
}

func (t *testStore) GetCustomDictionaryByName(ctx context.Context, name string) (CustomDictionary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key, ok := t.dictByName[name]
	if !ok {
		return CustomDictionary{}, notFound("custom_dictionary", name)
	}
	return t.dicts[key], nil
}

func (t *testStore) ListCustomDictionaries(ctx context.Context) ([]CustomDictionary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]CustomDictionary, 0, len(t.dicts))
	for _, x := range t.dicts {
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (t *testStore) UpdateCustomDictionary(ctx context.Context, arg UpdateCustomDictionaryParams) (CustomDictionary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.dicts[uuidKey(arg.ID)]
	if !ok {
		return CustomDictionary{}, notFound("custom_dictionary", "")
	}
	if other, taken := t.dictByName[arg.Name]; taken && other != uuidKey(arg.ID) {
		return CustomDictionary{}, duplicate("custom_dictionary", arg.Name)
	}
	delete(t.dictByName, row.Name)
	row.Name = arg.Name
	row.Description = arg.Description
	row.XmlContent = arg.XmlContent
	row.IsActive = arg.IsActive
	row.UpdatedAt = timestamptz(t.now())
	t.dicts[uuidKey(arg.ID)] = row
	t.dictByName[arg.Name] = uuidKey(arg.ID)
	return row, nil
}

func (t *testStore) DeleteCustomDictionary(ctx context.Context, id pgtype.UUID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	row, ok := t.dicts[uuidKey(id)]
	if !ok {
		return notFound("custom_dictionary", "")
	}
	delete(t.dicts, uuidKey(id))
	delete(t.dictByName, row.Name)
	return nil
}

// ----- helpers --------------------------------------------------

// cloneBytes deep-copies a JSONB-shaped byte slice. The store
// returns rows by value, but the body is a slice header — without
// a copy a caller could mutate the stored row by writing through
// the returned slice. The cost is small; the safety is large.
func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// lessUUID compares two pgtype.UUID values byte-by-byte. Used by
// GetSubscriberByName to mirror the production query's
// ORDER BY id LIMIT 1 semantics.
func lessUUID(a, b pgtype.UUID) bool {
	for i := range a.Bytes {
		if a.Bytes[i] != b.Bytes[i] {
			return a.Bytes[i] < b.Bytes[i]
		}
	}
	return false
}
