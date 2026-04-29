// Package store is the persistence wrapper for the OCS Testbench.
//
// The Store interface is the single coupling point between the
// application layers (engine, API, executions) and the database. Two
// constructors are provided:
//
//   - NewStore  — wraps the sqlc-generated Queries over a pgx
//     connection pool for production use; maps the pgconn error
//     codes for not-found, unique-violation, and foreign-key
//     violations onto the package-level sentinels.
//
//   - NewTestStore — returns an in-memory implementation of the
//     same interface, backed by maps. The test store enforces the
//     same uniqueness and foreign-key invariants as the schema, so
//     unit tests can exercise the contract without a live database.
//
// All errors returned by either implementation can be compared via
// errors.Is against ErrNotFound, ErrDuplicateName, and ErrForeignKey.

package store

import (
	"context"

	"github.com/eddiecarpenter/ocs-testbench/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// Type aliases — re-export the sqlc-generated row models so callers
// of the store package do not have to import internal/store/sqlc
// directly. The aliases let the wrapper layer evolve later (e.g. to
// hand-rolled domain types) without breaking call sites.
type (
	// Peer is one row of the peer table.
	Peer = sqlc.Peer
	// Subscriber is one row of the subscriber table.
	Subscriber = sqlc.Subscriber
	// AVPTemplate is one row of the avp_template table.
	AVPTemplate = sqlc.AvpTemplate
	// Scenario is one row of the scenario table.
	Scenario = sqlc.Scenario
	// CustomDictionary is one row of the custom_dictionary table.
	CustomDictionary = sqlc.CustomDictionary

	// InsertSubscriberParams is the argument shape for
	// Store.InsertSubscriber. Re-exported so callers do not need to
	// import internal/store/sqlc.
	InsertSubscriberParams = sqlc.InsertSubscriberParams
	// UpdateSubscriberParams is the argument shape for
	// Store.UpdateSubscriber.
	UpdateSubscriberParams = sqlc.UpdateSubscriberParams
	// UpdateScenarioParams is the argument shape for
	// Store.UpdateScenario.
	UpdateScenarioParams = sqlc.UpdateScenarioParams
	// UpdateCustomDictionaryParams is the argument shape for
	// Store.UpdateCustomDictionary.
	UpdateCustomDictionaryParams = sqlc.UpdateCustomDictionaryParams
)

// Store is the persistence interface for the OCS Testbench data
// model. Every later application layer (Diameter engine, REST API,
// execution runtime) couples against this interface, never against
// a concrete database driver. Two implementations satisfy it:
// NewStore (production, pgx) and NewTestStore (in-memory).
//
// Errors returned by every method can be compared against the
// package-level sentinels (ErrNotFound, ErrDuplicateName,
// ErrForeignKey) via errors.Is.
type Store interface {
	// ----- peer ----------------------------------------------------

	// InsertPeer creates a new peer with the given name and JSONB
	// body. The id is generated server-side. Returns
	// ErrDuplicateName if name is already taken.
	InsertPeer(ctx context.Context, name string, body []byte) (Peer, error)
	// GetPeer fetches a peer by id. Returns ErrNotFound if missing.
	GetPeer(ctx context.Context, id pgtype.UUID) (Peer, error)
	// GetPeerByName fetches a peer by its unique name. Returns
	// ErrNotFound if no peer has that name.
	GetPeerByName(ctx context.Context, name string) (Peer, error)
	// ListPeers returns every peer in name order.
	ListPeers(ctx context.Context) ([]Peer, error)
	// UpdatePeer rewrites the name and body of a peer. Returns
	// ErrNotFound if id does not resolve, ErrDuplicateName if name
	// is taken by another peer.
	UpdatePeer(ctx context.Context, id pgtype.UUID, name string, body []byte) (Peer, error)
	// DeletePeer removes a peer. Returns ErrNotFound if id does not
	// resolve, ErrForeignKey if any scenario still references it.
	DeletePeer(ctx context.Context, id pgtype.UUID) error

	// ----- subscriber ---------------------------------------------

	// InsertSubscriber creates a new subscriber. Subscriber names
	// are not unique; the call cannot fail with ErrDuplicateName.
	InsertSubscriber(ctx context.Context, arg InsertSubscriberParams) (Subscriber, error)
	// GetSubscriber fetches a subscriber by id. Returns ErrNotFound
	// if missing.
	GetSubscriber(ctx context.Context, id pgtype.UUID) (Subscriber, error)
	// GetSubscriberByName fetches the lexicographically-first
	// subscriber with the given display name. Returns ErrNotFound if
	// no subscriber has that name. Subscriber names are not unique;
	// callers that need an exhaustive lookup should use
	// ListSubscribers and filter the result.
	GetSubscriberByName(ctx context.Context, name string) (Subscriber, error)
	// ListSubscribers returns every subscriber in name order.
	ListSubscribers(ctx context.Context) ([]Subscriber, error)
	// UpdateSubscriber rewrites a subscriber. Returns ErrNotFound
	// if id does not resolve.
	UpdateSubscriber(ctx context.Context, arg UpdateSubscriberParams) (Subscriber, error)
	// DeleteSubscriber removes a subscriber. Returns ErrNotFound if
	// id does not resolve.
	DeleteSubscriber(ctx context.Context, id pgtype.UUID) error

	// ----- avp_template -------------------------------------------

	// InsertAVPTemplate creates a new AVP template. Returns
	// ErrDuplicateName if name is already taken.
	InsertAVPTemplate(ctx context.Context, name string, body []byte) (AVPTemplate, error)
	// GetAVPTemplate fetches a template by id. Returns ErrNotFound
	// if missing.
	GetAVPTemplate(ctx context.Context, id pgtype.UUID) (AVPTemplate, error)
	// GetAVPTemplateByName fetches a template by its unique name.
	// Returns ErrNotFound if no template has that name.
	GetAVPTemplateByName(ctx context.Context, name string) (AVPTemplate, error)
	// ListAVPTemplates returns every template in name order.
	ListAVPTemplates(ctx context.Context) ([]AVPTemplate, error)
	// UpdateAVPTemplate rewrites the name and body of a template.
	// Returns ErrNotFound if id does not resolve, ErrDuplicateName
	// if name collides.
	UpdateAVPTemplate(ctx context.Context, id pgtype.UUID, name string, body []byte) (AVPTemplate, error)
	// DeleteAVPTemplate removes a template. Returns ErrNotFound if
	// id does not resolve, ErrForeignKey if any scenario still
	// references it.
	DeleteAVPTemplate(ctx context.Context, id pgtype.UUID) error

	// ----- scenario -----------------------------------------------

	// InsertScenario creates a new scenario referencing the given
	// template and peer. Returns ErrDuplicateName if name is taken,
	// ErrForeignKey if templateID or peerID does not resolve.
	InsertScenario(ctx context.Context, name string, templateID, peerID pgtype.UUID, body []byte) (Scenario, error)
	// GetScenario fetches a scenario by id. Returns ErrNotFound if
	// missing.
	GetScenario(ctx context.Context, id pgtype.UUID) (Scenario, error)
	// GetScenarioByName fetches a scenario by its unique name.
	// Returns ErrNotFound if no scenario has that name.
	GetScenarioByName(ctx context.Context, name string) (Scenario, error)
	// ListScenarios returns every scenario in name order.
	ListScenarios(ctx context.Context) ([]Scenario, error)
	// UpdateScenario rewrites a scenario. Returns ErrNotFound if id
	// does not resolve, ErrDuplicateName if name collides,
	// ErrForeignKey if templateID or peerID does not resolve.
	UpdateScenario(ctx context.Context, arg UpdateScenarioParams) (Scenario, error)
	// DeleteScenario removes a scenario. Returns ErrNotFound if id
	// does not resolve.
	DeleteScenario(ctx context.Context, id pgtype.UUID) error

	// ----- custom_dictionary --------------------------------------

	// InsertCustomDictionary creates a new custom Diameter
	// dictionary fragment. Returns ErrDuplicateName if name is
	// taken.
	InsertCustomDictionary(ctx context.Context, name string, description pgtype.Text, xmlContent string, isActive bool) (CustomDictionary, error)
	// GetCustomDictionary fetches a dictionary by id. Returns
	// ErrNotFound if missing.
	GetCustomDictionary(ctx context.Context, id pgtype.UUID) (CustomDictionary, error)
	// GetCustomDictionaryByName fetches a dictionary by its unique
	// name. Returns ErrNotFound if no dictionary has that name.
	GetCustomDictionaryByName(ctx context.Context, name string) (CustomDictionary, error)
	// ListCustomDictionaries returns every dictionary in name order.
	ListCustomDictionaries(ctx context.Context) ([]CustomDictionary, error)
	// UpdateCustomDictionary rewrites a dictionary. Returns
	// ErrNotFound if id does not resolve, ErrDuplicateName if name
	// collides.
	UpdateCustomDictionary(ctx context.Context, arg UpdateCustomDictionaryParams) (CustomDictionary, error)
	// DeleteCustomDictionary removes a dictionary. Returns
	// ErrNotFound if id does not resolve.
	DeleteCustomDictionary(ctx context.Context, id pgtype.UUID) error

	// ----- lifecycle ----------------------------------------------

	// Close releases any resources held by the store. Production
	// stores close the underlying connection pool; the test store
	// is a no-op. After Close every other method returns an error.
	Close()
}

// uuidKey converts a pgtype.UUID to a comparable map key. The
// underlying Bytes is already a [16]byte, but capturing it here keeps
// every test-store map literal at one call site.
func uuidKey(id pgtype.UUID) [16]byte { return id.Bytes }
