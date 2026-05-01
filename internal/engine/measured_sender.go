package engine

import (
	"context"
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
)

// MeasuredSender wraps a messaging.Sender to capture per-request timing and
// size metrics without modifying the underlying Diameter stack.
//
// MeasuredSender is bound to a single peer at creation time — the session is
// inherently single-peer, so binding the peerName here avoids threading it
// through every call.
//
// MeasuredSender is NOT a messaging.Sender — it has a different Send
// signature (no peerName parameter) because the peer is fixed at construction.
type MeasuredSender struct {
	inner    messaging.Sender
	peerName string
}

// newMeasuredSender creates a MeasuredSender wrapping inner, bound to
// peerName. Panics on a nil inner (programming error — the session must
// always have a real or test Sender).
func newMeasuredSender(inner messaging.Sender, peerName string) *MeasuredSender {
	if inner == nil {
		panic("engine.newMeasuredSender: inner Sender must not be nil")
	}
	return &MeasuredSender{inner: inner, peerName: peerName}
}

// Send calls the underlying messaging.Sender with the bound peerName,
// capturing wall-clock timestamps around the call to compute RTT and
// populate SendMetrics.
//
// When the inner Sender returns an error, the metrics are still populated
// with timing data (CCA and ResponseSize are zero/nil) and Metrics.Err
// records the error. The error is also returned as the second return value
// for standard Go error handling.
func (m *MeasuredSender) Send(ctx context.Context, req *messaging.CCR) (SendResult, error) {
	sentAt := time.Now()
	cca, err := m.inner.Send(ctx, m.peerName, req)
	receivedAt := time.Now()

	metrics := SendMetrics{
		SentAt:     sentAt,
		ReceivedAt: receivedAt,
		RTT:        receivedAt.Sub(sentAt),
		// RequestSize: the encoded CCR wire size is not accessible without
		// re-encoding via the dictionary; left as 0 intentionally.
		RequestSize: 0,
		Err:         err,
	}

	if cca != nil && cca.Raw != nil {
		metrics.ResponseSize = int(cca.Raw.Header.MessageLength)
		metrics.ResultCode = cca.ResultCode
	}

	return SendResult{CCA: cca, Metrics: metrics}, err
}
