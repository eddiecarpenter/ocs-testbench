package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
)

// fakeSender is a test implementation of messaging.Sender that returns a
// canned CCA response (or an error) without touching the Diameter stack.
type fakeSender struct {
	cca *messaging.CCA
	err error
	// delay simulates network latency so RTT > 0 in tests.
	delay time.Duration
}

func (f *fakeSender) Send(ctx context.Context, _ string, _ *messaging.CCR) (*messaging.CCA, error) {
	if f.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(f.delay):
		}
	}
	// Check context even without a delay so tests can cancel mid-execution.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return f.cca, f.err
}

// newFakeCCA builds a minimal CCA with the supplied ResultCode and a
// synthesised *diam.Message whose MessageLength is set to msgLen.
func newFakeCCA(resultCode uint32, msgLen uint32) *messaging.CCA {
	hdr := &diam.Header{MessageLength: msgLen}
	raw := &diam.Message{Header: hdr}
	return &messaging.CCA{
		ResultCode: resultCode,
		FUIAction:  -1,
		Raw:        raw,
	}
}

// ---------- NewSessionContext --------------------------------------------

func TestNewSessionContext_Fields(t *testing.T) {
	sender := &fakeSender{cca: newFakeCCA(2001, 200)}
	sc := NewSessionContext("peer-a", "testbench.example.com", sender, ModeInteractive)

	require.NotNil(t, sc)
	assert.Equal(t, "peer-a", sc.PeerName)
	assert.Equal(t, ModeInteractive, sc.Mode)
	assert.Equal(t, StateActive, sc.State)
	assert.Equal(t, uint32(0), sc.CCRequestNumber)
}

func TestNewSessionContext_SessionIDFormat(t *testing.T) {
	sender := &fakeSender{cca: newFakeCCA(2001, 200)}
	sc := NewSessionContext("peer-a", "testbench.example.com", sender, ModeContinuous)

	// Session-Id must start with the origin host and contain two numeric
	// parts separated by semicolons: <host>;<hi32>;<lo32>.
	parts := strings.Split(sc.SessionID, ";")
	require.Len(t, parts, 3, "Session-Id must have 3 semicolon-separated parts: got %q", sc.SessionID)
	assert.Equal(t, "testbench.example.com", parts[0])
	assert.NotEmpty(t, parts[1], "hi32 part must not be empty")
	assert.NotEmpty(t, parts[2], "lo32 part must not be empty")
}

func TestNewSessionContext_SessionIDsAreUnique(t *testing.T) {
	sender := &fakeSender{cca: newFakeCCA(2001, 200)}
	sc1 := NewSessionContext("peer-a", "host.example.com", sender, ModeInteractive)
	sc2 := NewSessionContext("peer-a", "host.example.com", sender, ModeInteractive)
	assert.NotEqual(t, sc1.SessionID, sc2.SessionID, "each NewSessionContext must produce a unique Session-Id")
}

func TestNewSessionContext_VarsSeeded(t *testing.T) {
	sender := &fakeSender{cca: newFakeCCA(2001, 200)}
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeInteractive)

	val, ok := sc.Vars["CC_REQUEST_NUMBER"]
	require.True(t, ok, "Vars must be pre-seeded with CC_REQUEST_NUMBER")
	assert.Equal(t, uint32(0), val)
}

func TestNewSessionContext_SenderWrapped(t *testing.T) {
	sender := &fakeSender{cca: newFakeCCA(2001, 200)}
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeInteractive)

	// sender field must not be nil and must wrap the supplied Sender.
	require.NotNil(t, sc.sender, "sender must be wrapped in a MeasuredSender")
	assert.Equal(t, sender, sc.sender.inner)
	assert.Equal(t, "peer-a", sc.sender.peerName)
}

// ---------- MeasuredSender.Send ------------------------------------------

func TestMeasuredSender_Send_SuccessMetrics(t *testing.T) {
	delay := 2 * time.Millisecond
	fakeCCA := newFakeCCA(2001, 512)
	sender := &fakeSender{cca: fakeCCA, delay: delay}
	ms := newMeasuredSender(sender, "peer-a")

	result, err := ms.Send(context.Background(), &messaging.CCR{})

	require.NoError(t, err)
	require.NotNil(t, result.CCA)
	assert.Equal(t, uint32(2001), result.Metrics.ResultCode)
	assert.Equal(t, 512, result.Metrics.ResponseSize)
	assert.Nil(t, result.Metrics.Err)
	assert.GreaterOrEqual(t, result.Metrics.RTT, delay, "RTT must be >= simulated delay")
	assert.False(t, result.Metrics.SentAt.IsZero(), "SentAt must be set")
	assert.False(t, result.Metrics.ReceivedAt.IsZero(), "ReceivedAt must be set")
	assert.True(t, result.Metrics.ReceivedAt.After(result.Metrics.SentAt) ||
		result.Metrics.ReceivedAt.Equal(result.Metrics.SentAt),
		"ReceivedAt must be >= SentAt")
}

func TestMeasuredSender_Send_ErrorMetrics(t *testing.T) {
	sendErr := errors.New("peer disconnected")
	sender := &fakeSender{err: sendErr}
	ms := newMeasuredSender(sender, "peer-a")

	result, err := ms.Send(context.Background(), &messaging.CCR{})

	assert.ErrorIs(t, err, sendErr)
	assert.Nil(t, result.CCA)
	assert.Equal(t, sendErr, result.Metrics.Err)
	assert.Equal(t, uint32(0), result.Metrics.ResultCode)
	assert.Equal(t, 0, result.Metrics.ResponseSize)
	assert.False(t, result.Metrics.SentAt.IsZero())
}

func TestMeasuredSender_FakeSender_BehavesIdentically(t *testing.T) {
	// AC: a fake messaging.Sender returns identical StepResult shape as a real Sender.
	// This test confirms that MeasuredSender captures metrics the same way
	// regardless of whether the wrapped Sender talks to a real Diameter peer
	// or returns a canned response.
	fakeCCA := newFakeCCA(4012, 128)
	sender := &fakeSender{cca: fakeCCA}
	ms := newMeasuredSender(sender, "any-peer")

	result, err := ms.Send(context.Background(), &messaging.CCR{CCRequestType: messaging.CCRTypeInitial})

	require.NoError(t, err)
	assert.Equal(t, uint32(4012), result.Metrics.ResultCode)
	assert.Equal(t, 128, result.Metrics.ResponseSize)
}

// ---------- SessionContext.Send ------------------------------------------

func TestSessionContext_Send_DelegatesToMeasuredSender(t *testing.T) {
	fakeCCA := newFakeCCA(2001, 256)
	sender := &fakeSender{cca: fakeCCA}
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeInteractive)

	result, err := sc.Send(context.Background(), &messaging.CCR{})

	require.NoError(t, err)
	require.NotNil(t, result.CCA)
	assert.Equal(t, uint32(2001), result.Metrics.ResultCode)
}

// ---------- RecordStep / AllMetrics / MetricsByStep / Summary ------------

func TestSessionContext_RecordStep_AccumulatesInOrder(t *testing.T) {
	sc := NewSessionContext("peer-a", "host.example.com", &fakeSender{}, ModeInteractive)

	r1 := SendResult{Metrics: SendMetrics{RTT: 10 * time.Millisecond, ResultCode: 2001}}
	r2 := SendResult{Metrics: SendMetrics{RTT: 20 * time.Millisecond, ResultCode: 4012}}
	sc.RecordStep(r1)
	sc.RecordStep(r2)

	all := sc.AllMetrics()
	require.Len(t, all, 2)
	assert.Equal(t, 10*time.Millisecond, all[0].RTT)
	assert.Equal(t, 20*time.Millisecond, all[1].RTT)
}

func TestSessionContext_AllMetrics_EmptyOnFreshContext(t *testing.T) {
	sc := NewSessionContext("peer-a", "host.example.com", &fakeSender{}, ModeInteractive)
	assert.Nil(t, sc.AllMetrics())
}

func TestSessionContext_AllMetrics_ReturnsCopy(t *testing.T) {
	sc := NewSessionContext("peer-a", "host.example.com", &fakeSender{}, ModeInteractive)
	sc.RecordStep(SendResult{Metrics: SendMetrics{RTT: 5 * time.Millisecond}})

	copy1 := sc.AllMetrics()
	sc.RecordStep(SendResult{Metrics: SendMetrics{RTT: 10 * time.Millisecond}})
	copy2 := sc.AllMetrics()

	assert.Len(t, copy1, 1, "first snapshot must not be affected by later RecordStep calls")
	assert.Len(t, copy2, 2)
}

func TestSessionContext_MetricsByStep_ReturnsCorrectEntry(t *testing.T) {
	sc := NewSessionContext("peer-a", "host.example.com", &fakeSender{}, ModeInteractive)
	sc.RecordStep(SendResult{Metrics: SendMetrics{RTT: 10 * time.Millisecond}})
	sc.RecordStep(SendResult{Metrics: SendMetrics{RTT: 20 * time.Millisecond}})

	m0, ok0 := sc.MetricsByStep(0)
	m1, ok1 := sc.MetricsByStep(1)
	_, ok2 := sc.MetricsByStep(2)

	assert.True(t, ok0)
	assert.Equal(t, 10*time.Millisecond, m0.RTT)
	assert.True(t, ok1)
	assert.Equal(t, 20*time.Millisecond, m1.RTT)
	assert.False(t, ok2, "out-of-range index must return false")
}

func TestSessionContext_MetricsByStep_NegativeIndex(t *testing.T) {
	sc := NewSessionContext("peer-a", "host.example.com", &fakeSender{}, ModeInteractive)
	sc.RecordStep(SendResult{Metrics: SendMetrics{RTT: 5 * time.Millisecond}})

	_, ok := sc.MetricsByStep(-1)
	assert.False(t, ok, "negative index must return false")
}

func TestSessionContext_Summary_AggregateCounts(t *testing.T) {
	sc := NewSessionContext("peer-a", "host.example.com", &fakeSender{}, ModeInteractive)
	sc.RecordStep(SendResult{Metrics: SendMetrics{RTT: 10 * time.Millisecond, ResultCode: 2001}})
	sc.RecordStep(SendResult{Metrics: SendMetrics{RTT: 30 * time.Millisecond, ResultCode: 2001}})
	sc.RecordStep(SendResult{Metrics: SendMetrics{RTT: 20 * time.Millisecond, Err: errors.New("timeout")}})

	s := sc.Summary()

	assert.Equal(t, 3, s.TotalRequests)
	assert.Equal(t, 2, s.SuccessCount)
	assert.Equal(t, 1, s.FailureCount)
	assert.Equal(t, 10*time.Millisecond, s.MinRTT)
	assert.Equal(t, 30*time.Millisecond, s.MaxRTT)
	// avg = (10+30+20)/3 = 20ms
	assert.Equal(t, 20*time.Millisecond, s.AvgRTT)
}

func TestSessionContext_Summary_EmptyContext(t *testing.T) {
	sc := NewSessionContext("peer-a", "host.example.com", &fakeSender{}, ModeInteractive)
	s := sc.Summary()
	assert.Zero(t, s.TotalRequests)
	assert.Zero(t, s.MinRTT)
	assert.Zero(t, s.MaxRTT)
	assert.Zero(t, s.AvgRTT)
}

func TestSessionContext_Summary_SingleStep(t *testing.T) {
	sc := NewSessionContext("peer-a", "host.example.com", &fakeSender{}, ModeInteractive)
	sc.RecordStep(SendResult{Metrics: SendMetrics{RTT: 15 * time.Millisecond, ResultCode: 2001}})

	s := sc.Summary()
	assert.Equal(t, 1, s.TotalRequests)
	assert.Equal(t, 1, s.SuccessCount)
	assert.Equal(t, 0, s.FailureCount)
	assert.Equal(t, 15*time.Millisecond, s.MinRTT)
	assert.Equal(t, 15*time.Millisecond, s.MaxRTT)
	assert.Equal(t, 15*time.Millisecond, s.AvgRTT)
}

// ---------- MeasuredSender — nil panic guard -----------------------------

func TestNewMeasuredSender_NilPanics(t *testing.T) {
	assert.Panics(t, func() {
		newMeasuredSender(nil, "peer-a")
	}, "newMeasuredSender with nil inner must panic")
}
