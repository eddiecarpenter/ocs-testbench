package template

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseTemplateBody_RoundTrip verifies that a TemplateBody
// marshals to JSON and back to an equal value.
func TestParseTemplateBody_RoundTrip(t *testing.T) {
	original := TemplateBody{
		Version: 1,
		AVPs: []AVPNode{
			{Name: "Origin-Host", Value: "{{ORIGIN_HOST}}"},
			{
				Name:     "Service-Information",
				VendorID: 10415,
				AVPs: []AVPNode{
					{
						Name: "PS-Information",
						AVPs: []AVPNode{
							{Name: "3GPP-Charging-Id", Value: "{{CHARGING_ID}}"},
						},
					},
				},
			},
		},
		MSCC: []MSCCTemplateBlock{
			{
				RatingGroup:       100,
				ServiceIdentifier: 1,
				Requested:         "{{RG100_REQUESTED}}",
				Used:              "{{RG100_USED}}",
			},
		},
		StaticValues:    map[string]string{"ORIGIN_HOST": "ocs-testbench.local"},
		GeneratedValues: []string{"SESSION_ID", "CHARGING_ID"},
	}

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	got, err := ParseTemplateBody(raw)
	require.NoError(t, err)

	assert.Equal(t, original, got)
}

// TestParseTemplateBody_RejectsUnknownVersion ensures version != 1
// returns UNKNOWN_TEMPLATE_VERSION.
func TestParseTemplateBody_RejectsUnknownVersion(t *testing.T) {
	tests := []struct {
		name    string
		version int
	}{
		{name: "version 0", version: 0},
		{name: "version 2", version: 2},
		{name: "version 99", version: 99},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{"version": tc.version, "avps": []any{}})
			require.NoError(t, err)

			_, err = ParseTemplateBody(raw)
			require.Error(t, err)

			var te *TemplateError
			require.ErrorAs(t, err, &te)
			assert.Equal(t, ErrCodeUnknownTemplateVersion, te.Code)
		})
	}
}

// TestParseTemplateBody_RejectsMixedModeNode ensures a node with both
// value and child AVPs returns INVALID_AVP_NODE.
func TestParseTemplateBody_RejectsMixedModeNode(t *testing.T) {
	body := TemplateBody{
		Version: 1,
		AVPs: []AVPNode{
			{
				Name:  "Bad-AVP",
				Value: "some-value",
				AVPs:  []AVPNode{{Name: "Child", Value: "x"}},
			},
		},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	_, err = ParseTemplateBody(raw)
	require.Error(t, err)

	var te *TemplateError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, ErrCodeInvalidAVPNode, te.Code)
}

// TestParseTemplateBody_RejectsMixedModeNode_Nested ensures the
// mixed-mode check recurses into grouped children.
func TestParseTemplateBody_RejectsMixedModeNode_Nested(t *testing.T) {
	body := TemplateBody{
		Version: 1,
		AVPs: []AVPNode{
			{
				Name: "Outer-Grouped",
				AVPs: []AVPNode{
					{
						Name:  "Inner-Bad",
						Value: "x",
						AVPs:  []AVPNode{{Name: "Grandchild", Value: "y"}},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	_, err = ParseTemplateBody(raw)
	require.Error(t, err)

	var te *TemplateError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, ErrCodeInvalidAVPNode, te.Code)
}

// TestParseTemplateBody_AcceptsLeafNode ensures a leaf node (value
// set, no children) parses without error.
func TestParseTemplateBody_AcceptsLeafNode(t *testing.T) {
	body := TemplateBody{
		Version: 1,
		AVPs:    []AVPNode{{Name: "Origin-Host", Value: "ocs.local"}},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	got, err := ParseTemplateBody(raw)
	require.NoError(t, err)
	assert.Equal(t, 1, len(got.AVPs))
	assert.Equal(t, "Origin-Host", got.AVPs[0].Name)
}

// TestParseTemplateBody_AcceptsGroupedNode ensures a grouped node
// (children set, no value) parses without error.
func TestParseTemplateBody_AcceptsGroupedNode(t *testing.T) {
	body := TemplateBody{
		Version: 1,
		AVPs: []AVPNode{
			{
				Name:     "Service-Information",
				VendorID: 10415,
				AVPs: []AVPNode{
					{Name: "PS-Information", AVPs: []AVPNode{}},
				},
			},
		},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	got, err := ParseTemplateBody(raw)
	require.NoError(t, err)
	assert.Equal(t, int64(10415), got.AVPs[0].VendorID)
}

// TestParseTemplateBody_AcceptsVendorIDPermutations checks that
// vendor-id presence on various node shapes is preserved in the
// round-trip.
func TestParseTemplateBody_AcceptsVendorIDPermutations(t *testing.T) {
	tests := []struct {
		name     string
		node     AVPNode
		wantVID  int64
	}{
		{
			name:    "leaf with vendor-id",
			node:    AVPNode{Name: "SomeAVP", VendorID: 10415, Value: "x"},
			wantVID: 10415,
		},
		{
			name:    "grouped with vendor-id",
			node:    AVPNode{Name: "SomeGrouped", VendorID: 10415, AVPs: []AVPNode{{Name: "Child", Value: "y"}}},
			wantVID: 10415,
		},
		{
			name:    "leaf without vendor-id",
			node:    AVPNode{Name: "BaseAVP", Value: "z"},
			wantVID: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := TemplateBody{Version: 1, AVPs: []AVPNode{tc.node}}
			raw, err := json.Marshal(body)
			require.NoError(t, err)

			got, err := ParseTemplateBody(raw)
			require.NoError(t, err)
			assert.Equal(t, tc.wantVID, got.AVPs[0].VendorID)
		})
	}
}

// TestParseTemplateBody_AcceptsEmptyTemplate ensures a minimal valid
// template (version 1, no AVPs) parses without error.
func TestParseTemplateBody_AcceptsEmptyTemplate(t *testing.T) {
	raw := []byte(`{"version":1}`)
	got, err := ParseTemplateBody(raw)
	require.NoError(t, err)
	assert.Equal(t, 1, got.Version)
	assert.Empty(t, got.AVPs)
}

// TestParseTemplateBody_RejectsInvalidJSON ensures non-JSON input
// returns a parse error.
func TestParseTemplateBody_RejectsInvalidJSON(t *testing.T) {
	_, err := ParseTemplateBody([]byte(`not-json`))
	require.Error(t, err)
}
