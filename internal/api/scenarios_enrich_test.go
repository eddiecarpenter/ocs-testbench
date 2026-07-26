package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/template"
)

// stubDict is a minimal template.Dictionary for exercising enrichAvpTree. It
// reports every name as unknown so codes/vendorIds are left untouched — the
// enrich pass still runs (dict != nil), which is what we want to test.
type stubDict struct{}

func (stubDict) Lookup(string) (template.AVPMetadata, error) {
	return template.AVPMetadata{}, assert.AnError
}

// TestEnrichAvpTree_PreservesEmptyGroup proves the enrich round-trip keeps an
// empty grouped AVP as children:[] (grouped) rather than dropping the key and
// collapsing it into a leaf. This is the round-trip half of the empty-group fix.
func TestEnrichAvpTree_PreservesEmptyGroup(t *testing.T) {
	in := json.RawMessage(`[
		{"name":"Service-Information","code":873,"children":[
			{"name":"USSD-Information","code":20600,"children":[]}
		]},
		{"name":"USSD-Detail","code":20654,"valueRef":"USSD_DETAIL"}
	]`)

	out := enrichAvpTree(in, stubDict{})

	var nodes []map[string]any
	require.NoError(t, json.Unmarshal(out, &nodes))
	require.Len(t, nodes, 2)

	// Root grouped node keeps its children.
	svcInfo := nodes[0]
	svcChildren, ok := svcInfo["children"].([]any)
	require.True(t, ok, "Service-Information should keep children")
	require.Len(t, svcChildren, 1)

	// Nested empty group keeps children:[] (present but empty).
	ussdInfo := svcChildren[0].(map[string]any)
	nested, hasChildren := ussdInfo["children"]
	assert.True(t, hasChildren, "empty USSD-Information must keep its children key")
	arr, _ := nested.([]any)
	assert.Empty(t, arr, "USSD-Information children should be an empty array")

	// The leaf must NOT gain a children key.
	leaf := nodes[1]
	_, leafHasChildren := leaf["children"]
	assert.False(t, leafHasChildren, "leaf AVP must not carry a children key")
	assert.Equal(t, "USSD_DETAIL", leaf["valueRef"])
}
