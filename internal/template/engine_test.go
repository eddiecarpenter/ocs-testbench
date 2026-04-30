package template

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// ---- fake dictionary for engine tests ----------------------------

// engineFakeDict maps AVP name to metadata. Tests populate it with
// only the AVPs they reference; unknown names return UNKNOWN_AVP.
type engineFakeDict struct {
	avps map[string]AVPMetadata
}

func newEngineDict(entries ...AVPMetadata) *engineFakeDict {
	d := &engineFakeDict{avps: make(map[string]AVPMetadata)}
	for _, e := range entries {
		d.avps[e.DataType] = e // indexed by name below
	}
	return d
}

// newEngineDict2 creates a fake dictionary from name→metadata pairs.
func newEngineDict2(avps map[string]AVPMetadata) *engineFakeDict {
	return &engineFakeDict{avps: avps}
}

func (d *engineFakeDict) Lookup(name string) (AVPMetadata, error) {
	meta, ok := d.avps[name]
	if !ok {
		return AVPMetadata{}, errUnknownAVP(name)
	}
	return meta, nil
}

// standardDict returns a fake dictionary with common AVP definitions
// used across multiple tests.
func standardDict() *engineFakeDict {
	return newEngineDict2(map[string]AVPMetadata{
		"Origin-Host":         {Code: 264, DataType: "UTF8String"},
		"Origin-Realm":        {Code: 296, DataType: "UTF8String"},
		"Service-Context-Id":  {Code: 461, DataType: "UTF8String"},
		"CC-Request-Type":     {Code: 416, DataType: "Enumerated"},
		"CC-Request-Number":   {Code: 415, DataType: "Unsigned32"},
		"Event-Timestamp":     {Code: 55, DataType: "Time"},
		"Rating-Group":        {Code: 432, DataType: "Unsigned32"},
		"Charging-Id":         {Code: 2, VendorID: 10415, DataType: "Unsigned32"},
		"Service-Information": {Code: 873, VendorID: 10415, DataType: "Grouped", Grouped: true},
		"PS-Information":      {Code: 874, VendorID: 10415, DataType: "Grouped", Grouped: true},
		"3GPP-Charging-Id":    {Code: 2, VendorID: 10415, DataType: "OctetString"},
	})
}

// ---- helper to render and assert ----------------------------------

func render(t *testing.T, d Dictionary, tree []AVPNode, values map[string]any, sm ServiceModel, ut UnitType, mscc []MSCCTemplateBlock) []*diam.AVP {
	t.Helper()
	e := NewEngine()
	input := EngineInput{
		Tree:         tree,
		MSCC:         mscc,
		Values:       values,
		Dictionary:   d,
		ServiceModel: sm,
		UnitType:     ut,
	}
	avps, err := e.Render(context.Background(), input)
	require.NoError(t, err)
	return avps
}

// findAVP searches for the first AVP with the given code in the list.
func findAVP(avps []*diam.AVP, code uint32) *diam.AVP {
	for _, a := range avps {
		if a.Code == code {
			return a
		}
	}
	return nil
}

// countAVP counts AVPs with the given code.
func countAVP(avps []*diam.AVP, code uint32) int {
	n := 0
	for _, a := range avps {
		if a.Code == code {
			n++
		}
	}
	return n
}

// ---- AC-1: static template → correct encoded tree ----------------

func TestEngine_Render_AC1_StaticTemplate_CorrectTypes(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{
		"Origin-Host":       {Code: 264, DataType: "UTF8String"},
		"CC-Request-Number": {Code: 415, DataType: "Unsigned32"},
		"Event-Timestamp":   {Code: 55, DataType: "Time"},
	})

	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tree := []AVPNode{
		{Name: "Origin-Host", Value: "ocs.local"},
		{Name: "CC-Request-Number", Value: "3"},
		{Name: "Event-Timestamp", Value: "{{TIMESTAMP}}"},
	}
	avps := render(t, d, tree, map[string]any{"TIMESTAMP": ts}, ServiceModelRoot, UnitTypeTime, nil)

	require.Len(t, avps, 3)
	assert.Equal(t, uint32(264), avps[0].Code)
	assert.Equal(t, datatype.UTF8String("ocs.local"), avps[0].Data)
	assert.Equal(t, uint32(415), avps[1].Code)
	assert.Equal(t, datatype.Unsigned32(3), avps[1].Data)
	assert.Equal(t, uint32(55), avps[2].Code)
	assert.Equal(t, datatype.Time(ts), avps[2].Data)
}

// ---- AC-2: placeholder substitution ------------------------------

func TestEngine_Render_AC2_PlaceholderSubstitution(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{
		"Origin-Host":  {Code: 264, DataType: "UTF8String"},
		"Rating-Group": {Code: 432, DataType: "Unsigned32"},
	})
	tree := []AVPNode{
		{Name: "Origin-Host", Value: "{{ORIGIN}}"},
		{Name: "Rating-Group", Value: "{{RG}}"},
	}
	avps := render(t, d, tree, map[string]any{
		"ORIGIN": "my-host.local",
		"RG":     uint32(100),
	}, ServiceModelRoot, UnitTypeTime, nil)

	require.Len(t, avps, 2)
	assert.Equal(t, datatype.UTF8String("my-host.local"), avps[0].Data)
	assert.Equal(t, datatype.Unsigned32(100), avps[1].Data)
}

// ---- AC-3: unresolved placeholder --------------------------------

func TestEngine_Render_AC3_UnresolvedPlaceholder_ReturnsError(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{
		"Origin-Host": {Code: 264, DataType: "UTF8String"},
	})
	e := NewEngine()
	_, err := e.Render(context.Background(), EngineInput{
		Tree:       []AVPNode{{Name: "Origin-Host", Value: "{{MISSING_TOKEN}}"}},
		Values:     map[string]any{},
		Dictionary: d,
	})
	require.Error(t, err)

	var te *TemplateError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, ErrCodeUnresolvedPlaceholder, te.Code)
	assert.Contains(t, te.Detail, "MISSING_TOKEN")
}

// ---- AC-4: static value overridden by value map (loader; covered by Task 1)

// ---- AC-5: generated values auto-substituted (loader; covered by Task 1)

// ---- AC-6: unknown AVP name → UNKNOWN_AVP -----------------------

func TestEngine_Render_AC6_UnknownAVPName_ReturnsError(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{}) // empty — every lookup fails
	e := NewEngine()
	_, err := e.Render(context.Background(), EngineInput{
		Tree:       []AVPNode{{Name: "Unknown-AVP", Value: "x"}},
		Values:     map[string]any{},
		Dictionary: d,
	})
	require.Error(t, err)

	var te *TemplateError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, ErrCodeUnknownAVP, te.Code)
	assert.Contains(t, te.Detail, "Unknown-AVP")
}

// ---- AC-7: vendor-id propagates to children ----------------------

func TestEngine_Render_AC7_VendorIDPropagatestoChildren(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{
		"Service-Information": {Code: 873, DataType: "Grouped", Grouped: true},
		"PS-Information":      {Code: 874, DataType: "Grouped", Grouped: true},
		"3GPP-Charging-Id":    {Code: 2, DataType: "OctetString"},
	})
	tree := []AVPNode{
		{
			Name:     "Service-Information",
			VendorID: 10415,
			AVPs: []AVPNode{
				{
					Name: "PS-Information",
					AVPs: []AVPNode{
						// 3GPP-Charging-Id has no explicit VendorID — inherits 10415
						{Name: "3GPP-Charging-Id", Value: "{{CID}}"},
					},
				},
			},
		},
	}
	e := NewEngine()
	avps, err := e.Render(context.Background(), EngineInput{
		Tree:         tree,
		Values:       map[string]any{"CID": "AABBCCDD"},
		Dictionary:   d,
		ServiceModel: ServiceModelRoot,
	})
	require.NoError(t, err)
	require.Len(t, avps, 1)

	// Service-Information → vendor 10415
	assert.Equal(t, uint32(10415), avps[0].VendorID)

	// PS-Information (child) inherits 10415
	siGrouped, ok := avps[0].Data.(*diam.GroupedAVP)
	require.True(t, ok, "expected Service-Information to be GroupedAVP")
	require.Len(t, siGrouped.AVP, 1)
	assert.Equal(t, uint32(10415), siGrouped.AVP[0].VendorID)

	// 3GPP-Charging-Id (grandchild) inherits 10415
	psGrouped, ok := siGrouped.AVP[0].Data.(*diam.GroupedAVP)
	require.True(t, ok, "expected PS-Information to be GroupedAVP")
	require.Len(t, psGrouped.AVP, 1)
	assert.Equal(t, uint32(10415), psGrouped.AVP[0].VendorID)
}

// ---- AC-8: child explicit vendor-id overrides parent -------------

func TestEngine_Render_AC8_ChildVendorIDOverridesParent(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{
		"Outer": {Code: 1001, DataType: "Grouped", Grouped: true},
		"Inner": {Code: 1002, DataType: "UTF8String"},
	})
	tree := []AVPNode{
		{
			Name:     "Outer",
			VendorID: 10415,
			AVPs: []AVPNode{
				// Inner explicitly sets a different VendorID.
				{Name: "Inner", VendorID: 9999, Value: "x"},
			},
		},
	}
	e := NewEngine()
	avps, err := e.Render(context.Background(), EngineInput{
		Tree:         tree,
		Values:       map[string]any{},
		Dictionary:   d,
		ServiceModel: ServiceModelRoot,
	})
	require.NoError(t, err)
	require.Len(t, avps, 1)
	assert.Equal(t, uint32(10415), avps[0].VendorID, "Outer should keep its own vendor-id")

	grouped, ok := avps[0].Data.(*diam.GroupedAVP)
	require.True(t, ok)
	require.Len(t, grouped.AVP, 1)
	assert.Equal(t, uint32(9999), grouped.AVP[0].VendorID, "Inner must override with its own vendor-id")
}

// ---- AC-9: multi-MSCC, each block independent -------------------

func TestEngine_Render_AC9_MultiMSCC_IndependentBlocks(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{
		"Origin-Host": {Code: 264, DataType: "UTF8String"},
	})
	mscc := []MSCCTemplateBlock{
		{RatingGroup: 100, ServiceIdentifier: 1, Requested: "{{RG100_REQ}}", Used: "{{RG100_USED}}"},
		{RatingGroup: 200, ServiceIdentifier: 2, Requested: "{{RG200_REQ}}", Used: "{{RG200_USED}}"},
	}
	values := map[string]any{
		"CC_REQUEST_TYPE": uint32(2), // UPDATE — both RSU (if > 0) and USU always
		"RG100_REQ":       "1024",
		"RG100_USED":      "512",
		"RG200_REQ":       "2048",
		"RG200_USED":      "1024",
	}

	e := NewEngine()
	avps, err := e.Render(context.Background(), EngineInput{
		Tree:         []AVPNode{{Name: "Origin-Host", Value: "ocs.local"}},
		MSCC:         mscc,
		Values:       values,
		Dictionary:   d,
		ServiceModel: ServiceModelMultiMSCC,
		UnitType:     UnitTypeOctet,
	})
	require.NoError(t, err)

	// MSI=1 + 2 MSCC blocks + 1 tree AVP
	msccCount := countAVP(avps, avp.MultipleServicesCreditControl)
	assert.Equal(t, 2, msccCount, "expected 2 independent MSCC blocks")

	msiAVP := findAVP(avps, avp.MultipleServicesIndicator)
	require.NotNil(t, msiAVP)
	assert.Equal(t, datatype.Enumerated(1), msiAVP.Data, "MSI must be 1 for multi-mscc")

	// Each MSCC block is independently built — verify they have different RG children.
	var msccBlocks []*diam.AVP
	for _, a := range avps {
		if a.Code == avp.MultipleServicesCreditControl {
			msccBlocks = append(msccBlocks, a)
		}
	}
	require.Len(t, msccBlocks, 2)

	firstRG := findChildAVP(msccBlocks[0], avp.RatingGroup)
	secondRG := findChildAVP(msccBlocks[1], avp.RatingGroup)
	require.NotNil(t, firstRG)
	require.NotNil(t, secondRG)
	assert.NotEqual(t, firstRG.Data, secondRG.Data, "MSCC blocks must have different Rating-Group values")
}

// findChildAVP returns the first child AVP with the given code from a
// Grouped parent AVP, or nil if not found.
func findChildAVP(parent *diam.AVP, code uint32) *diam.AVP {
	g, ok := parent.Data.(*diam.GroupedAVP)
	if !ok {
		return nil
	}
	for _, child := range g.AVP {
		if child.Code == code {
			return child
		}
	}
	return nil
}

// ---- AC-10: step-override isolation (also covered by Task 1 loader test)

// ---- AC-11: type mismatch ----------------------------------------

func TestEngine_Render_AC11_TypeMismatch_ReturnsError(t *testing.T) {
	tests := []struct {
		name     string
		dataType string
		value    any
	}{
		{name: "string where Unsigned32 expected", dataType: "Unsigned32", value: "not-a-number"},
		{name: "string where Unsigned64 expected", dataType: "Unsigned64", value: "not-a-number"},
		{name: "string where Integer32 expected", dataType: "Integer32", value: "not-a-number"},
		{name: "bad IP for Address", dataType: "Address", value: "not-an-ip"},
		{name: "bad RFC3339 for Time", dataType: "Time", value: "not-a-timestamp"},
		{name: "string where Enumerated expected", dataType: "Enumerated", value: "not-a-number"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := newEngineDict2(map[string]AVPMetadata{
				"TestAVP": {Code: 9999, DataType: tc.dataType},
			})
			e := NewEngine()
			_, err := e.Render(context.Background(), EngineInput{
				Tree:       []AVPNode{{Name: "TestAVP", Value: "{{VAL}}"}},
				Values:     map[string]any{"VAL": tc.value},
				Dictionary: d,
			})
			require.Error(t, err, "expected error for %s", tc.name)

			var te *TemplateError
			require.ErrorAs(t, err, &te)
			assert.Equal(t, ErrCodeEncodingTypeMismatch, te.Code)
		})
	}
}

// ---- AC-12: loader round-trip (also covered by Task 1 loader test)

// ---- AC-13: hand-built EngineInput equals loader-fed ------------

// TestEngine_Render_AC13_HandBuiltInputEqualLoaderFed verifies that
// an EngineInput constructed directly (not from the store) produces
// the same AVP tree as one produced via the Loader. This proves the
// engine has no store dependency.
func TestEngine_Render_AC13_HandBuiltInputEqualLoaderFed(t *testing.T) {
	// Shared tree and value map.
	tree := []AVPNode{
		{Name: "Origin-Host", Value: "{{ORIGIN}}"},
		{Name: "CC-Request-Number", Value: "{{CCR_NUM}}"},
	}
	values := map[string]any{
		"ORIGIN":  "host.test",
		"CCR_NUM": uint32(0),
	}
	d := newEngineDict2(map[string]AVPMetadata{
		"Origin-Host":       {Code: 264, DataType: "UTF8String"},
		"CC-Request-Number": {Code: 415, DataType: "Unsigned32"},
	})

	// Path A: hand-built EngineInput.
	handBuilt := EngineInput{
		Tree:         tree,
		Values:       values,
		Dictionary:   d,
		ServiceModel: ServiceModelRoot,
		UnitType:     UnitTypeTime,
	}

	// Path B: loader-fed EngineInput (via store round-trip).
	s := store.NewTestStore()
	body := TemplateBody{
		Version:      1,
		AVPs:         tree,
		StaticValues: map[string]string{}, // no statics — values come from context
	}
	raw, _ := json.Marshal(body)
	row, err := s.InsertAVPTemplate(context.Background(), "ac13-template", raw)
	require.NoError(t, err)

	loader := NewLoader(s, &fakeGenerator{}, d)
	loaderFed, err := loader.Load(context.Background(), row.ID, ScenarioCtx{
		Variables:    map[string]string{"ORIGIN": "host.test"},
		UnitType:     UnitTypeTime,
		ServiceModel: ServiceModelRoot,
	})
	require.NoError(t, err)
	// Patch CCR_NUM value (loader doesn't populate it, hand-build does).
	loaderFed.Values["CCR_NUM"] = uint32(0)

	eng := NewEngine()
	handBuiltAVPs, err := eng.Render(context.Background(), handBuilt)
	require.NoError(t, err)

	loaderAVPs, err := eng.Render(context.Background(), loaderFed)
	require.NoError(t, err)

	require.Equal(t, len(handBuiltAVPs), len(loaderAVPs), "both paths must produce the same number of AVPs")
	for i := range handBuiltAVPs {
		assert.Equal(t, handBuiltAVPs[i].Code, loaderAVPs[i].Code,
			"AVP code mismatch at index %d", i)
		assert.Equal(t, handBuiltAVPs[i].Data, loaderAVPs[i].Data,
			"AVP data mismatch at index %d", i)
	}
}

// ---- service model tests ----------------------------------------

// TestEngine_Render_ServiceModelSingleMSCC verifies that single-mscc
// emits exactly one MSCC block and MSI=0.
func TestEngine_Render_ServiceModelSingleMSCC(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{})
	mscc := []MSCCTemplateBlock{
		{RatingGroup: 10, Requested: "{{REQ}}", Used: "{{USED}}"},
	}
	values := map[string]any{
		"CC_REQUEST_TYPE": uint32(2), // UPDATE
		"REQ":             "500",
		"USED":            "250",
	}

	e := NewEngine()
	avps, err := e.Render(context.Background(), EngineInput{
		MSCC: mscc, Values: values, Dictionary: d,
		ServiceModel: ServiceModelSingleMSCC, UnitType: UnitTypeTime,
	})
	require.NoError(t, err)

	msiAVP := findAVP(avps, avp.MultipleServicesIndicator)
	require.NotNil(t, msiAVP, "MSI must be present for single-mscc")
	assert.Equal(t, datatype.Enumerated(0), msiAVP.Data, "MSI must be 0 for single-mscc")

	assert.Equal(t, 1, countAVP(avps, avp.MultipleServicesCreditControl))
}

// TestEngine_Render_ServiceModelRoot verifies root mode: no MSI, no
// MSCC, RSU/USU at root level.
func TestEngine_Render_ServiceModelRoot(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{})
	mscc := []MSCCTemplateBlock{
		{Requested: "{{REQ}}", Used: "{{USED}}"},
	}
	values := map[string]any{
		"CC_REQUEST_TYPE": uint32(2), // UPDATE: RSU if > 0, USU always
		"REQ":             "300",
		"USED":            "150",
	}

	e := NewEngine()
	avps, err := e.Render(context.Background(), EngineInput{
		MSCC: mscc, Values: values, Dictionary: d,
		ServiceModel: ServiceModelRoot, UnitType: UnitTypeTime,
	})
	require.NoError(t, err)

	// No MSI in root mode.
	assert.Nil(t, findAVP(avps, avp.MultipleServicesIndicator), "root mode must not emit MSI")

	// No MSCC in root mode.
	assert.Equal(t, 0, countAVP(avps, avp.MultipleServicesCreditControl), "root mode must not emit MSCC")

	// RSU and USU present at root.
	assert.NotNil(t, findAVP(avps, avp.RequestedServiceUnit), "RSU must be present in root mode")
	assert.NotNil(t, findAVP(avps, avp.UsedServiceUnit), "USU must be present in root mode")
}

// TestEngine_Render_PresenceRules_Initial verifies INITIAL: RSU if > 0, no USU.
func TestEngine_Render_PresenceRules_Initial(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{})
	mscc := []MSCCTemplateBlock{
		{RatingGroup: 1, Requested: "{{REQ}}", Used: "{{USED}}"},
	}
	values := map[string]any{
		"CC_REQUEST_TYPE": uint32(1), // INITIAL
		"REQ":             "100",
		"USED":            "50",
	}

	e := NewEngine()
	avps, err := e.Render(context.Background(), EngineInput{
		MSCC: mscc, Values: values, Dictionary: d,
		ServiceModel: ServiceModelMultiMSCC, UnitType: UnitTypeOctet,
	})
	require.NoError(t, err)

	msccAVP := findAVP(avps, avp.MultipleServicesCreditControl)
	require.NotNil(t, msccAVP)
	// INITIAL: RSU must be present.
	assert.NotNil(t, findChildAVP(msccAVP, avp.RequestedServiceUnit), "RSU must be present for INITIAL")
	// INITIAL: USU must NOT be present.
	assert.Nil(t, findChildAVP(msccAVP, avp.UsedServiceUnit), "USU must be absent for INITIAL")
}

// TestEngine_Render_PresenceRules_Terminate verifies TERMINATE: no RSU, USU always.
func TestEngine_Render_PresenceRules_Terminate(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{})
	mscc := []MSCCTemplateBlock{
		{RatingGroup: 1, Requested: "{{REQ}}", Used: "{{USED}}"},
	}
	values := map[string]any{
		"CC_REQUEST_TYPE": uint32(3), // TERMINATE
		"REQ":             "100",
		"USED":            "50",
	}

	e := NewEngine()
	avps, err := e.Render(context.Background(), EngineInput{
		MSCC: mscc, Values: values, Dictionary: d,
		ServiceModel: ServiceModelMultiMSCC, UnitType: UnitTypeTime,
	})
	require.NoError(t, err)

	msccAVP := findAVP(avps, avp.MultipleServicesCreditControl)
	require.NotNil(t, msccAVP)
	// TERMINATE: RSU must NOT be present.
	assert.Nil(t, findChildAVP(msccAVP, avp.RequestedServiceUnit), "RSU must be absent for TERMINATE")
	// TERMINATE: USU must be present.
	assert.NotNil(t, findChildAVP(msccAVP, avp.UsedServiceUnit), "USU must be present for TERMINATE")
}

// ---- unit type encoding tests -----------------------------------

// TestEngine_Render_UnitType_Octet verifies CC-Total-Octets inner AVP
// for OCTET unit type.
func TestEngine_Render_UnitType_Octet(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{})
	mscc := []MSCCTemplateBlock{
		{RatingGroup: 1, Requested: "{{REQ}}"},
	}
	values := map[string]any{"CC_REQUEST_TYPE": uint32(1), "REQ": "4096"}

	e := NewEngine()
	avps, err := e.Render(context.Background(), EngineInput{
		MSCC: mscc, Values: values, Dictionary: d,
		ServiceModel: ServiceModelMultiMSCC, UnitType: UnitTypeOctet,
	})
	require.NoError(t, err)

	msccAVP := findAVP(avps, avp.MultipleServicesCreditControl)
	require.NotNil(t, msccAVP)
	rsu := findChildAVP(msccAVP, avp.RequestedServiceUnit)
	require.NotNil(t, rsu, "RSU must be present")

	// RSU child must be CC-Total-Octets (Unsigned64).
	rsuGrouped, ok := rsu.Data.(*diam.GroupedAVP)
	require.True(t, ok)
	require.Len(t, rsuGrouped.AVP, 1)
	assert.Equal(t, uint32(avp.CCTotalOctets), rsuGrouped.AVP[0].Code)
	assert.Equal(t, datatype.Unsigned64(4096), rsuGrouped.AVP[0].Data)
}

// TestEngine_Render_UnitType_Time verifies CC-Time inner AVP.
func TestEngine_Render_UnitType_Time(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{})
	mscc := []MSCCTemplateBlock{{Requested: "{{REQ}}"}}
	values := map[string]any{"CC_REQUEST_TYPE": uint32(1), "REQ": "60"}

	e := NewEngine()
	avps, err := e.Render(context.Background(), EngineInput{
		MSCC: mscc, Values: values, Dictionary: d,
		ServiceModel: ServiceModelMultiMSCC, UnitType: UnitTypeTime,
	})
	require.NoError(t, err)

	msccAVP := findAVP(avps, avp.MultipleServicesCreditControl)
	rsu := findChildAVP(msccAVP, avp.RequestedServiceUnit)
	require.NotNil(t, rsu)
	rsuGrouped := rsu.Data.(*diam.GroupedAVP)
	assert.Equal(t, uint32(avp.CCTime), rsuGrouped.AVP[0].Code)
	assert.Equal(t, datatype.Unsigned32(60), rsuGrouped.AVP[0].Data)
}

// ---- mixed-placeholder string substitution ----------------------

func TestEngine_Render_MixedPlaceholderString(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{
		"Service-Context-Id": {Code: 461, DataType: "UTF8String"},
	})
	tree := []AVPNode{
		{Name: "Service-Context-Id", Value: "32251@{{REALM}}"},
	}
	avps := render(t, d, tree, map[string]any{"REALM": "3gpp.org"}, ServiceModelRoot, UnitTypeTime, nil)
	require.Len(t, avps, 1)
	assert.Equal(t, datatype.UTF8String("32251@3gpp.org"), avps[0].Data)
}

// ---- error: unresolved in nested grouped node -------------------

func TestEngine_Render_UnresolvedInGroupedNode_ReturnsError(t *testing.T) {
	d := newEngineDict2(map[string]AVPMetadata{
		"Outer": {Code: 1001, DataType: "Grouped", Grouped: true},
		"Inner": {Code: 1002, DataType: "UTF8String"},
	})
	e := NewEngine()
	_, err := e.Render(context.Background(), EngineInput{
		Tree: []AVPNode{
			{Name: "Outer", AVPs: []AVPNode{
				{Name: "Inner", Value: "{{NO_SUCH_TOKEN}}"},
			}},
		},
		Values:     map[string]any{},
		Dictionary: d,
	})
	require.Error(t, err)

	var te *TemplateError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, ErrCodeUnresolvedPlaceholder, te.Code)
}

// TestEngine_Render_EncodeAll_BasicTypes verifies all supported
// datatype encodings round-trip correctly.
func TestEngine_Render_EncodeAll_BasicTypes(t *testing.T) {
	ts := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		avpName  string
		code     uint32
		dataType string
		value    any
		want     datatype.Type
	}{
		{avpName: "U32", code: 1, dataType: "Unsigned32", value: uint32(42), want: datatype.Unsigned32(42)},
		{avpName: "U64", code: 2, dataType: "Unsigned64", value: uint64(100), want: datatype.Unsigned64(100)},
		{avpName: "I32", code: 3, dataType: "Integer32", value: int32(-5), want: datatype.Integer32(-5)},
		{avpName: "I64", code: 4, dataType: "Integer64", value: int64(-1), want: datatype.Integer64(-1)},
		{avpName: "UTF8", code: 5, dataType: "UTF8String", value: "hello", want: datatype.UTF8String("hello")},
		{avpName: "Enum", code: 6, dataType: "Enumerated", value: int32(3), want: datatype.Enumerated(3)},
		{avpName: "Time", code: 7, dataType: "Time", value: ts, want: datatype.Time(ts)},
		{avpName: "DI", code: 8, dataType: "DiameterIdentity", value: "host.local", want: datatype.DiameterIdentity("host.local")},
	}
	for _, tc := range tests {
		t.Run(tc.avpName, func(t *testing.T) {
			d := newEngineDict2(map[string]AVPMetadata{
				tc.avpName: {Code: tc.code, DataType: tc.dataType},
			})
			e := NewEngine()
			avps, err := e.Render(context.Background(), EngineInput{
				Tree:       []AVPNode{{Name: tc.avpName, Value: fmt.Sprintf("{{V_%s}}", tc.avpName)}},
				Values:     map[string]any{fmt.Sprintf("V_%s", tc.avpName): tc.value},
				Dictionary: d,
			})
			require.NoError(t, err)
			require.Len(t, avps, 1)
			assert.Equal(t, tc.want, avps[0].Data)
		})
	}
}
