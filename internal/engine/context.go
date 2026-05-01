package engine

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
)

// SessionContext is the stateful execution thread for a single scenario run.
// It binds the execution to a peer, carries context variables across steps,
// wraps the Sender in a MeasuredSender for metrics capture, and accumulates
// per-step metrics with a query API.
//
// SessionContext is safe for concurrent reads via AllMetrics, MetricsByStep,
// and Summary. The step executor (which runs sequentially) writes to
// CCRequestNumber and Vars without additional locking — both are owned by the
// single execution goroutine. External callers that read State and Vars
// concurrently must hold mu.RLock explicitly if they require a consistent
// snapshot; the query methods (AllMetrics, MetricsByStep, Summary) acquire
// the lock internally.
type SessionContext struct {
	// PeerName is the Diameter peer this session is bound to.
	PeerName string
	// SessionID is the RFC 6733 Session-Id generated at context creation:
	// <OriginHost>;<uuid-hi32>;<uuid-lo32>.
	SessionID string
	// CCRequestNumber is the CC-Request-Number for the next CCR. Starts at
	// 0 (INITIAL_REQUEST per RFC 4006). The step executor increments this
	// after each send.
	CCRequestNumber uint32
	// Mode controls interactive vs. continuous execution.
	Mode ExecutionMode
	// State tracks the lifecycle of this session.
	State SessionState
	// Vars is the live substitution map carried across steps. It is
	// pre-seeded with CC_REQUEST_NUMBER = 0 at creation and updated by
	// extractions and derived-value evaluations during execution.
	Vars map[string]any

	// sender is the metrics-capturing decorator over the real Sender. The
	// raw messaging.Sender is not directly accessible outside the package.
	sender *MeasuredSender

	// metrics is the ordered per-step accumulator. Appended via RecordStep;
	// read via AllMetrics, MetricsByStep, Summary.
	metrics []SendMetrics

	// mu guards the metrics slice (written by RecordStep during the
	// sequential step walk; read by the query methods which may be called
	// from a different goroutine, e.g. an API handler).
	mu sync.RWMutex
}

// NewSessionContext creates a SessionContext bound to peerName, wrapping s in
// a MeasuredSender. It generates a Session-Id in RFC 6733 format:
//
//	<OriginHost>;<uuid-hi32>;<uuid-lo32>
//
// using the two halves of a freshly generated UUID v4. Vars is initialised to
// an empty map seeded with CC_REQUEST_NUMBER = 0, and CCRequestNumber starts
// at 0 (the INITIAL_REQUEST value per RFC 4006 §8.3).
func NewSessionContext(peerName, originHost string, s messaging.Sender, mode ExecutionMode) *SessionContext {
	id := uuid.New()
	hi32 := binary.BigEndian.Uint32(id[0:4])
	lo32 := binary.BigEndian.Uint32(id[4:8])
	sessionID := fmt.Sprintf("%s;%d;%d", originHost, hi32, lo32)

	return &SessionContext{
		PeerName:        peerName,
		SessionID:       sessionID,
		CCRequestNumber: 0,
		Mode:            mode,
		State:           StateActive,
		Vars:            map[string]any{"CC_REQUEST_NUMBER": uint32(0)},
		sender:          newMeasuredSender(s, peerName),
	}
}

// Send delegates to the session's MeasuredSender, capturing metrics for the
// exchange. It is the canonical path for the step executor to send a CCR —
// the raw messaging.Sender is not accessible directly outside the package.
//
// Send does NOT call RecordStep; the step executor is responsible for calling
// RecordStep after inspecting the result (e.g. to allow the step to be
// skipped without recording empty metrics).
func (sc *SessionContext) Send(ctx context.Context, req *messaging.CCR) (SendResult, error) {
	return sc.sender.Send(ctx, req)
}

// RecordStep appends the metrics from a send result to the per-step
// accumulator. The step executor calls this after each non-skipped step.
//
// RecordStep is safe to call concurrently with the query methods.
func (sc *SessionContext) RecordStep(result SendResult) {
	sc.mu.Lock()
	sc.metrics = append(sc.metrics, result.Metrics)
	sc.mu.Unlock()
}

// AllMetrics returns a copy of the ordered per-step metrics accumulated so
// far. The slice is safe to inspect after the call; it will not be modified
// by subsequent RecordStep calls.
func (sc *SessionContext) AllMetrics() []SendMetrics {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	if len(sc.metrics) == 0 {
		return nil
	}
	out := make([]SendMetrics, len(sc.metrics))
	copy(out, sc.metrics)
	return out
}

// MetricsByStep returns the metrics for a specific step index (0-based,
// matching the position in AllMetrics). Returns false when the index is
// out of range.
func (sc *SessionContext) MetricsByStep(index int) (SendMetrics, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	if index < 0 || index >= len(sc.metrics) {
		return SendMetrics{}, false
	}
	return sc.metrics[index], true
}

// Summary computes aggregate metrics over all recorded steps.
//
// If no steps have been recorded, the returned MetricsSummary is zero-valued
// (MinRTT, MaxRTT, AvgRTT are all 0).
func (sc *SessionContext) Summary() MetricsSummary {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if len(sc.metrics) == 0 {
		return MetricsSummary{}
	}

	var (
		total    int
		success  int
		failure  int
		minRTT   = time.Duration(math.MaxInt64)
		maxRTT   time.Duration
		totalRTT time.Duration
	)

	for _, m := range sc.metrics {
		total++
		if m.Err != nil {
			failure++
		} else {
			success++
		}
		if m.RTT < minRTT {
			minRTT = m.RTT
		}
		if m.RTT > maxRTT {
			maxRTT = m.RTT
		}
		totalRTT += m.RTT
	}

	var avgRTT time.Duration
	if total > 0 {
		avgRTT = totalRTT / time.Duration(total)
	}

	if minRTT == time.Duration(math.MaxInt64) {
		minRTT = 0
	}

	return MetricsSummary{
		TotalRequests: total,
		SuccessCount:  success,
		FailureCount:  failure,
		MinRTT:        minRTT,
		MaxRTT:        maxRTT,
		AvgRTT:        avgRTT,
	}
}
