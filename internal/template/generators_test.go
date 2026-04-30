package template

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewGeneratorProvider_SessionID_Format verifies Session-Id
// follows the <Origin-Host>;<hi32>;<lo32> format.
func TestNewGeneratorProvider_SessionID_Format(t *testing.T) {
	g := NewGeneratorProvider("testbench.local", nil)

	val, err := g.Generate(TokenSessionID)
	require.NoError(t, err)

	sid, ok := val.(string)
	require.True(t, ok, "SESSION_ID must be a string")

	parts := strings.Split(sid, ";")
	assert.Equal(t, 3, len(parts), "Session-Id must have exactly 3 semicolon-delimited parts")
	assert.Equal(t, "testbench.local", parts[0])
	assert.NotEmpty(t, parts[1], "hi32 must not be empty")
	assert.NotEmpty(t, parts[2], "lo32 must not be empty")
}

// TestNewGeneratorProvider_SessionID_StableWithinProvider verifies
// SESSION_ID returns the same value on every call (refresh: once).
func TestNewGeneratorProvider_SessionID_StableWithinProvider(t *testing.T) {
	g := NewGeneratorProvider("testbench.local", nil)

	v1, err := g.Generate(TokenSessionID)
	require.NoError(t, err)
	v2, err := g.Generate(TokenSessionID)
	require.NoError(t, err)

	assert.Equal(t, v1, v2, "SESSION_ID must be stable within the same provider")
}

// TestNewGeneratorProvider_ChargingID_IsUint32 verifies CHARGING_ID
// returns a uint32 value.
func TestNewGeneratorProvider_ChargingID_IsUint32(t *testing.T) {
	g := NewGeneratorProvider("", nil)

	val, err := g.Generate(TokenChargingID)
	require.NoError(t, err)

	_, ok := val.(uint32)
	require.True(t, ok, "CHARGING_ID must be uint32")
}

// TestNewGeneratorProvider_ChargingID_StableWithinProvider verifies
// CHARGING_ID is stable within the same provider (refresh: once).
func TestNewGeneratorProvider_ChargingID_StableWithinProvider(t *testing.T) {
	g := NewGeneratorProvider("", nil)

	v1, _ := g.Generate(TokenChargingID)
	v2, _ := g.Generate(TokenChargingID)
	assert.Equal(t, v1, v2)
}

// TestNewGeneratorProvider_CCRequestNumber_Increments verifies
// CC_REQUEST_NUMBER starts at 0 and increments on each call.
func TestNewGeneratorProvider_CCRequestNumber_Increments(t *testing.T) {
	g := NewGeneratorProvider("", nil)

	for i := uint32(0); i < 5; i++ {
		val, err := g.Generate(TokenCCRequestNumber)
		require.NoError(t, err)
		n, ok := val.(uint32)
		require.True(t, ok, "CC_REQUEST_NUMBER must be uint32")
		assert.Equal(t, i, n, "expected request number %d, got %d", i, n)
	}
}

// TestNewGeneratorProvider_EventTimestamp_UsesNowFn verifies
// EVENT_TIMESTAMP calls the injected now function.
func TestNewGeneratorProvider_EventTimestamp_UsesNowFn(t *testing.T) {
	fixed := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	g := NewGeneratorProvider("", func() time.Time { return fixed })

	val, err := g.Generate(TokenEventTimestamp)
	require.NoError(t, err)

	ts, ok := val.(time.Time)
	require.True(t, ok, "EVENT_TIMESTAMP must be time.Time")
	assert.True(t, fixed.Equal(ts))
}

// TestNewGeneratorProvider_UnknownToken returns an error for
// unrecognised token names.
func TestNewGeneratorProvider_UnknownToken_ReturnsError(t *testing.T) {
	g := NewGeneratorProvider("", nil)

	_, err := g.Generate("UNKNOWN_TOKEN_XYZ")
	require.Error(t, err)
}

// TestNewGeneratorProvider_FallbackOriginHost verifies that an empty
// originHost falls back to "ocs-testbench.local".
func TestNewGeneratorProvider_FallbackOriginHost(t *testing.T) {
	g := NewGeneratorProvider("", nil)

	val, err := g.Generate(TokenSessionID)
	require.NoError(t, err)

	sid := val.(string)
	assert.True(t, strings.HasPrefix(sid, "ocs-testbench.local;"),
		"expected fallback origin host prefix, got %q", sid)
}
