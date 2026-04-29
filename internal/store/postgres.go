// Production Store implementation backed by a pgx connection pool
// and the sqlc-generated CRUD bindings under internal/store/sqlc.
//
// The wrapper is a thin facade: each method delegates to the
// corresponding sqlc.Queries method and translates the pgx and
// pgconn error shapes into the package-level sentinels (ErrNotFound,
// ErrDuplicateName, ErrForeignKey). The translation is the only
// reason this layer exists; without it every caller would have to
// import pgx and inspect SQLSTATE codes by hand.

package store

import (
	"context"
	"errors"

	"github.com/eddiecarpenter/ocs-testbench/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres SQLSTATE codes the wrapper recognises.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// pgStore implements Store over a pgxpool.Pool.
type pgStore struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewStore returns a Store implementation that wraps the provided
// pgx connection pool. The pool's lifetime is the caller's; Close
// on the returned store closes the pool.
//
// pool must be non-nil and connected; NewStore does not perform any
// IO of its own.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// Close releases the underlying connection pool. Subsequent calls
// against the store will fail; Close is idempotent.
func (s *pgStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// translate converts a raw pgx error into a Store sentinel where
// possible. It is the single point of error mapping for this layer:
//
//   - pgx.ErrNoRows         → ErrNotFound (wrapped in EntityError)
//   - SQLSTATE 23505        → ErrDuplicateName (wrapped)
//   - SQLSTATE 23503        → ErrForeignKey (wrapped)
//   - everything else       → returned untouched
//
// The entity and key are surfaced in the wrapper so log lines and
// test assertions can identify the offending row.
func translate(err error, entity, key string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound(entity, key)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return duplicate(entity, key)
		case pgForeignKeyViolation:
			return foreignKey(entity, key)
		}
	}
	return err
}

// uuidString renders a pgtype.UUID for error messages.
func uuidString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	// 8-4-4-4-12 grouping. Avoids importing google/uuid for a
	// single formatting helper.
	const hex = "0123456789abcdef"
	b := id.Bytes
	out := make([]byte, 0, 36)
	for i, v := range b {
		out = append(out, hex[v>>4], hex[v&0x0f])
		if i == 3 || i == 5 || i == 7 || i == 9 {
			out = append(out, '-')
		}
	}
	return string(out)
}

// ----- peer ------------------------------------------------------

func (s *pgStore) InsertPeer(ctx context.Context, name string, body []byte) (Peer, error) {
	row, err := s.queries.InsertPeer(ctx, name, body)
	return row, translate(err, "peer", name)
}

func (s *pgStore) GetPeer(ctx context.Context, id pgtype.UUID) (Peer, error) {
	row, err := s.queries.GetPeerByID(ctx, id)
	return row, translate(err, "peer", uuidString(id))
}

func (s *pgStore) GetPeerByName(ctx context.Context, name string) (Peer, error) {
	row, err := s.queries.GetPeerByName(ctx, name)
	return row, translate(err, "peer", name)
}

func (s *pgStore) ListPeers(ctx context.Context) ([]Peer, error) {
	rows, err := s.queries.ListPeers(ctx)
	return rows, translate(err, "peer", "")
}

func (s *pgStore) UpdatePeer(ctx context.Context, id pgtype.UUID, name string, body []byte) (Peer, error) {
	row, err := s.queries.UpdatePeer(ctx, id, name, body)
	return row, translate(err, "peer", uuidString(id))
}

func (s *pgStore) DeletePeer(ctx context.Context, id pgtype.UUID) error {
	tag, err := s.deleteWithTag(ctx, "peer", id, s.queries.DeletePeer)
	if err != nil {
		return err
	}
	if tag == 0 {
		return notFound("peer", uuidString(id))
	}
	return nil
}

// ----- subscriber -----------------------------------------------

func (s *pgStore) InsertSubscriber(ctx context.Context, arg InsertSubscriberParams) (Subscriber, error) {
	row, err := s.queries.InsertSubscriber(ctx, arg)
	return row, translate(err, "subscriber", arg.Name)
}

func (s *pgStore) GetSubscriber(ctx context.Context, id pgtype.UUID) (Subscriber, error) {
	row, err := s.queries.GetSubscriberByID(ctx, id)
	return row, translate(err, "subscriber", uuidString(id))
}

func (s *pgStore) GetSubscriberByName(ctx context.Context, name string) (Subscriber, error) {
	row, err := s.queries.GetSubscriberByName(ctx, name)
	return row, translate(err, "subscriber", name)
}

func (s *pgStore) ListSubscribers(ctx context.Context) ([]Subscriber, error) {
	rows, err := s.queries.ListSubscribers(ctx)
	return rows, translate(err, "subscriber", "")
}

func (s *pgStore) UpdateSubscriber(ctx context.Context, arg UpdateSubscriberParams) (Subscriber, error) {
	row, err := s.queries.UpdateSubscriber(ctx, arg)
	return row, translate(err, "subscriber", uuidString(arg.ID))
}

func (s *pgStore) DeleteSubscriber(ctx context.Context, id pgtype.UUID) error {
	tag, err := s.deleteWithTag(ctx, "subscriber", id, s.queries.DeleteSubscriber)
	if err != nil {
		return err
	}
	if tag == 0 {
		return notFound("subscriber", uuidString(id))
	}
	return nil
}

// ----- avp_template ---------------------------------------------

func (s *pgStore) InsertAVPTemplate(ctx context.Context, name string, body []byte) (AVPTemplate, error) {
	row, err := s.queries.InsertAVPTemplate(ctx, name, body)
	return row, translate(err, "avp_template", name)
}

func (s *pgStore) GetAVPTemplate(ctx context.Context, id pgtype.UUID) (AVPTemplate, error) {
	row, err := s.queries.GetAVPTemplateByID(ctx, id)
	return row, translate(err, "avp_template", uuidString(id))
}

func (s *pgStore) GetAVPTemplateByName(ctx context.Context, name string) (AVPTemplate, error) {
	row, err := s.queries.GetAVPTemplateByName(ctx, name)
	return row, translate(err, "avp_template", name)
}

func (s *pgStore) ListAVPTemplates(ctx context.Context) ([]AVPTemplate, error) {
	rows, err := s.queries.ListAVPTemplates(ctx)
	return rows, translate(err, "avp_template", "")
}

func (s *pgStore) UpdateAVPTemplate(ctx context.Context, id pgtype.UUID, name string, body []byte) (AVPTemplate, error) {
	row, err := s.queries.UpdateAVPTemplate(ctx, id, name, body)
	return row, translate(err, "avp_template", uuidString(id))
}

func (s *pgStore) DeleteAVPTemplate(ctx context.Context, id pgtype.UUID) error {
	tag, err := s.deleteWithTag(ctx, "avp_template", id, s.queries.DeleteAVPTemplate)
	if err != nil {
		return err
	}
	if tag == 0 {
		return notFound("avp_template", uuidString(id))
	}
	return nil
}

// ----- scenario -------------------------------------------------

func (s *pgStore) InsertScenario(ctx context.Context, name string, templateID, peerID pgtype.UUID, body []byte) (Scenario, error) {
	row, err := s.queries.InsertScenario(ctx, name, templateID, peerID, body)
	return row, translate(err, "scenario", name)
}

func (s *pgStore) GetScenario(ctx context.Context, id pgtype.UUID) (Scenario, error) {
	row, err := s.queries.GetScenarioByID(ctx, id)
	return row, translate(err, "scenario", uuidString(id))
}

func (s *pgStore) GetScenarioByName(ctx context.Context, name string) (Scenario, error) {
	row, err := s.queries.GetScenarioByName(ctx, name)
	return row, translate(err, "scenario", name)
}

func (s *pgStore) ListScenarios(ctx context.Context) ([]Scenario, error) {
	rows, err := s.queries.ListScenarios(ctx)
	return rows, translate(err, "scenario", "")
}

func (s *pgStore) UpdateScenario(ctx context.Context, arg UpdateScenarioParams) (Scenario, error) {
	row, err := s.queries.UpdateScenario(ctx, arg)
	return row, translate(err, "scenario", uuidString(arg.ID))
}

func (s *pgStore) DeleteScenario(ctx context.Context, id pgtype.UUID) error {
	tag, err := s.deleteWithTag(ctx, "scenario", id, s.queries.DeleteScenario)
	if err != nil {
		return err
	}
	if tag == 0 {
		return notFound("scenario", uuidString(id))
	}
	return nil
}

// ----- custom_dictionary ----------------------------------------

func (s *pgStore) InsertCustomDictionary(ctx context.Context, name string, description pgtype.Text, xmlContent string, isActive bool) (CustomDictionary, error) {
	row, err := s.queries.InsertCustomDictionary(ctx, name, description, xmlContent, isActive)
	return row, translate(err, "custom_dictionary", name)
}

func (s *pgStore) GetCustomDictionary(ctx context.Context, id pgtype.UUID) (CustomDictionary, error) {
	row, err := s.queries.GetCustomDictionaryByID(ctx, id)
	return row, translate(err, "custom_dictionary", uuidString(id))
}

func (s *pgStore) GetCustomDictionaryByName(ctx context.Context, name string) (CustomDictionary, error) {
	row, err := s.queries.GetCustomDictionaryByName(ctx, name)
	return row, translate(err, "custom_dictionary", name)
}

func (s *pgStore) ListCustomDictionaries(ctx context.Context) ([]CustomDictionary, error) {
	rows, err := s.queries.ListCustomDictionaries(ctx)
	return rows, translate(err, "custom_dictionary", "")
}

func (s *pgStore) UpdateCustomDictionary(ctx context.Context, arg UpdateCustomDictionaryParams) (CustomDictionary, error) {
	row, err := s.queries.UpdateCustomDictionary(ctx, arg)
	return row, translate(err, "custom_dictionary", uuidString(arg.ID))
}

func (s *pgStore) DeleteCustomDictionary(ctx context.Context, id pgtype.UUID) error {
	tag, err := s.deleteWithTag(ctx, "custom_dictionary", id, s.queries.DeleteCustomDictionary)
	if err != nil {
		return err
	}
	if tag == 0 {
		return notFound("custom_dictionary", uuidString(id))
	}
	return nil
}

// deleteWithTag wraps the sqlc Delete* helpers (which return only an
// error) with a "rows affected" probe so the wrapper can distinguish
// "row deleted" from "no row matched id". sqlc's :exec output discards
// the command tag, so the wrapper performs a separate COUNT-style
// check via a follow-up Get when the delete succeeded silently.
func (s *pgStore) deleteWithTag(
	ctx context.Context,
	entity string,
	id pgtype.UUID,
	delete func(context.Context, pgtype.UUID) error,
) (int64, error) {
	// Probe existence first so we can return ErrNotFound deterministically
	// without relying on the delete's command tag (which sqlc :exec
	// discards). The two-step is acceptable for the testbench's
	// volumes — concurrent deletes are not a contended path.
	if err := s.probe(ctx, entity, id); err != nil {
		return 0, err
	}
	if err := delete(ctx, id); err != nil {
		return 0, translate(err, entity, uuidString(id))
	}
	return 1, nil
}

// probe verifies a row exists; returns ErrNotFound otherwise. It
// dispatches by entity name so the wrapper does not have to plumb
// per-entity helpers through deleteWithTag.
func (s *pgStore) probe(ctx context.Context, entity string, id pgtype.UUID) error {
	var err error
	switch entity {
	case "peer":
		_, err = s.queries.GetPeerByID(ctx, id)
	case "subscriber":
		_, err = s.queries.GetSubscriberByID(ctx, id)
	case "avp_template":
		_, err = s.queries.GetAVPTemplateByID(ctx, id)
	case "scenario":
		_, err = s.queries.GetScenarioByID(ctx, id)
	case "custom_dictionary":
		_, err = s.queries.GetCustomDictionaryByID(ctx, id)
	default:
		// Caller bug — entity name not recognised. Surface as
		// ErrNotFound so it never silently passes.
		return notFound(entity, uuidString(id))
	}
	return translate(err, entity, uuidString(id))
}
