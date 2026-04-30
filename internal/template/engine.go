// Template engine — pure, stateless AVP tree renderer.
//
// The engine accepts an EngineInput (produced by the Loader or
// constructed directly by any caller) and returns []*diam.AVP ready
// for Feature #17's CCR builder to wrap in a CCR message.
//
// No store, no HTTP, no time.Now() calls. All non-determinism enters
// via the EngineInput's resolved value map.

package template

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
)

// Diameter CC-Request-Type constants (RFC 4006 §8.3). Used when
// reading CC_REQUEST_TYPE from the value map to apply §7 presence rules.
const (
	ccReqTypeInitial   = uint32(1)
	ccReqTypeUpdate    = uint32(2)
	ccReqTypeTerminate = uint32(3)
	ccReqTypeEvent     = uint32(4)
)

// singleTokenRe matches a string that is entirely one {{TOKEN}}
// placeholder. The token name may be any identifier (the architecture
// uses SCREAMING_SNAKE_CASE but the regex does not restrict case).
// Used to preserve the original Go type of the resolved value.
var singleTokenRe = regexp.MustCompile(`^\{\{([A-Za-z][A-Za-z0-9_]*)\}\}$`)

// anyTokenRe matches any {{TOKEN}} occurrence in a string for bulk
// string substitution when the value contains non-placeholder text.
var anyTokenRe = regexp.MustCompile(`\{\{([A-Za-z][A-Za-z0-9_]*)\}\}`)

// Engine is a pure, stateless AVP renderer. It has no fields; it is a
// receiver so callers can swap implementations in tests.
//
// Construct with NewEngine().
type Engine struct{}

// NewEngine returns a new Engine. It accepts no parameters; all
// non-determinism is supplied through the EngineInput at Render time.
func NewEngine() *Engine { return &Engine{} }

// Render processes the EngineInput and returns the fully-built
// []*diam.AVP tree ready for a CCR builder to consume.
//
// The result contains:
//   - The AVPs from input.Tree with placeholders substituted and
//     values encoded per the dictionary.
//   - The Multiple-Services-Indicator AVP and MSCC / RSU / USU blocks
//     per input.ServiceModel and input.UnitType.
//
// Render is safe for concurrent use — it modifies no shared state.
func (e *Engine) Render(_ context.Context, input EngineInput) ([]*diam.AVP, error) {
	// Build the main AVP tree from the template.
	tree, err := e.buildAVPList(input.Tree, input.Values, input.Dictionary, 0)
	if err != nil {
		return nil, err
	}

	// Build service-unit AVPs based on service model.
	serviceAVPs, err := e.buildServiceAVPs(input)
	if err != nil {
		return nil, err
	}

	return append(tree, serviceAVPs...), nil
}

// buildAVPList converts a slice of AVPNode into a slice of *diam.AVP,
// threading the effective vendor-id down the recursion.
func (e *Engine) buildAVPList(
	nodes []AVPNode,
	values map[string]any,
	d Dictionary,
	inheritedVendorID int64,
) ([]*diam.AVP, error) {
	result := make([]*diam.AVP, 0, len(nodes))
	for _, node := range nodes {
		a, err := e.buildAVPNode(node, values, d, inheritedVendorID)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, nil
}

// buildAVPNode processes one AVPNode: dictionary lookup, vendor-id
// inheritance, and either recursive grouped build or leaf encoding.
func (e *Engine) buildAVPNode(
	node AVPNode,
	values map[string]any,
	d Dictionary,
	inheritedVendorID int64,
) (*diam.AVP, error) {
	// Step 1 — dictionary lookup (AC-6: unknown name → UNKNOWN_AVP).
	meta, err := d.Lookup(node.Name)
	if err != nil {
		return nil, err
	}

	// Step 2 — vendor-id inheritance (AC-7, AC-8).
	effectiveVendorID := inheritedVendorID
	if node.VendorID != 0 {
		effectiveVendorID = node.VendorID
	}

	// Step 3 — AVP flags. Mbit is set for all AVPs; Vbit is set
	// automatically by diam.NewAVP when vendor > 0.
	flags := uint8(avp.Mbit)

	// Step 4 — grouped or leaf.
	if meta.Grouped || len(node.AVPs) > 0 {
		children, err := e.buildAVPList(node.AVPs, values, d, effectiveVendorID)
		if err != nil {
			return nil, err
		}
		grouped := &diam.GroupedAVP{AVP: children}
		return diam.NewAVP(meta.Code, flags, uint32(effectiveVendorID), grouped), nil
	}

	// Leaf — resolve placeholder and encode.
	resolved, err := e.resolveValue(node.Value, values)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeValue(node.Name, meta.DataType, resolved)
	if err != nil {
		return nil, err
	}
	return diam.NewAVP(meta.Code, flags, uint32(effectiveVendorID), encoded), nil
}

// buildServiceAVPs builds the MSI and MSCC / RSU / USU AVPs per §7.
func (e *Engine) buildServiceAVPs(input EngineInput) ([]*diam.AVP, error) {
	switch input.ServiceModel {
	case ServiceModelRoot:
		return e.buildRootServiceAVPs(input)
	case ServiceModelSingleMSCC:
		return e.buildSingleMSCCAVPs(input)
	case ServiceModelMultiMSCC:
		return e.buildMultiMSCCAVPs(input)
	default:
		// Unknown service model — no service AVPs.
		return nil, nil
	}
}

// buildRootServiceAVPs handles ServiceModelRoot: RSU/USU directly
// under the CCR root, no MSI, no MSCC block.
func (e *Engine) buildRootServiceAVPs(input EngineInput) ([]*diam.AVP, error) {
	if len(input.MSCC) == 0 {
		return nil, nil
	}
	reqType := requestTypeFromValues(input.Values)
	return e.buildServiceUnitPair(input.MSCC[0], input.Values, input.UnitType, reqType)
}

// buildSingleMSCCAVPs handles ServiceModelSingleMSCC: one MSCC block
// plus MSI=0 (MULTIPLE_SERVICES_NOT_SUPPORTED).
func (e *Engine) buildSingleMSCCAVPs(input EngineInput) ([]*diam.AVP, error) {
	var result []*diam.AVP

	// MSI=0
	msi := diam.NewAVP(avp.MultipleServicesIndicator, avp.Mbit, 0, datatype.Enumerated(0))
	result = append(result, msi)

	if len(input.MSCC) == 0 {
		return result, nil
	}

	reqType := requestTypeFromValues(input.Values)
	msccAVP, err := e.buildMSCCBlock(input.MSCC[0], input.Values, input.UnitType, reqType)
	if err != nil {
		return nil, err
	}
	result = append(result, msccAVP)
	return result, nil
}

// buildMultiMSCCAVPs handles ServiceModelMultiMSCC: one MSCC block
// per input.MSCC entry plus MSI=1 (MULTIPLE_SERVICES_SUPPORTED).
func (e *Engine) buildMultiMSCCAVPs(input EngineInput) ([]*diam.AVP, error) {
	var result []*diam.AVP

	// MSI=1
	msi := diam.NewAVP(avp.MultipleServicesIndicator, avp.Mbit, 0, datatype.Enumerated(1))
	result = append(result, msi)

	reqType := requestTypeFromValues(input.Values)
	for _, block := range input.MSCC {
		msccAVP, err := e.buildMSCCBlock(block, input.Values, input.UnitType, reqType)
		if err != nil {
			return nil, err
		}
		result = append(result, msccAVP)
	}
	return result, nil
}

// buildMSCCBlock constructs one Multiple-Services-Credit-Control AVP.
// Children: Rating-Group, Service-Identifier, RSU, USU per §7 rules.
func (e *Engine) buildMSCCBlock(
	block MSCCTemplateBlock,
	values map[string]any,
	unitType UnitType,
	reqType uint32,
) (*diam.AVP, error) {
	var children []*diam.AVP

	// Rating-Group (if declared)
	if block.RatingGroup > 0 {
		children = append(children,
			diam.NewAVP(avp.RatingGroup, avp.Mbit, 0, datatype.Unsigned32(block.RatingGroup)))
	}

	// Service-Identifier (if declared)
	if block.ServiceIdentifier > 0 {
		children = append(children,
			diam.NewAVP(avp.ServiceIdentifier, avp.Mbit, 0, datatype.Unsigned32(block.ServiceIdentifier)))
	}

	// RSU / USU per §7 presence rules (AC-9 independent per block)
	suPair, err := e.buildServiceUnitPair(block, values, unitType, reqType)
	if err != nil {
		return nil, err
	}
	children = append(children, suPair...)

	grouped := &diam.GroupedAVP{AVP: children}
	return diam.NewAVP(avp.MultipleServicesCreditControl, avp.Mbit, 0, grouped), nil
}

// buildServiceUnitPair returns the RSU and/or USU AVPs per §7 presence rules.
//
// Presence rules (docs/ARCHITECTURE.md §7):
//
//	INITIAL   — RSU if resolved > 0; USU never
//	UPDATE    — RSU if resolved > 0; USU always
//	TERMINATE — RSU never;           USU always
//	EVENT     — RSU if resolved > 0; USU if resolved > 0
//	unknown   — RSU if non-empty;    USU if non-empty
func (e *Engine) buildServiceUnitPair(
	block MSCCTemplateBlock,
	values map[string]any,
	unitType UnitType,
	reqType uint32,
) ([]*diam.AVP, error) {
	var result []*diam.AVP

	// Requested-Service-Unit
	emitRSU, resolvedReq, err := e.shouldEmitServiceUnit(block.Requested, values, reqType,
		/* allowTerminate */ false /* requireUpdate */, false)
	if err != nil {
		return nil, err
	}
	if emitRSU {
		rsu, err := buildServiceUnitAVP(avp.RequestedServiceUnit, unitType, resolvedReq)
		if err != nil {
			return nil, fmt.Errorf("RSU: %w", err)
		}
		result = append(result, rsu)
	}

	// Used-Service-Unit
	emitUSU, resolvedUsed, err := e.shouldEmitUSU(block.Used, values, reqType)
	if err != nil {
		return nil, err
	}
	if emitUSU {
		usu, err := buildServiceUnitAVP(avp.UsedServiceUnit, unitType, resolvedUsed)
		if err != nil {
			return nil, fmt.Errorf("USU: %w", err)
		}
		result = append(result, usu)
	}

	return result, nil
}

// shouldEmitServiceUnit checks whether RSU should be emitted and
// resolves its placeholder value.
func (e *Engine) shouldEmitServiceUnit(
	placeholder string,
	values map[string]any,
	reqType uint32,
	_ bool, // reserved
	_ bool, // reserved
) (bool, any, error) {
	if placeholder == "" {
		return false, nil, nil
	}
	// TERMINATE never includes RSU
	if reqType == ccReqTypeTerminate {
		return false, nil, nil
	}
	resolved, err := e.resolveValue(placeholder, values)
	if err != nil {
		return false, nil, err
	}
	// Emit RSU if resolved value > 0 (or if request type is unknown)
	if reqType == 0 {
		return true, resolved, nil
	}
	n, ok := toUint64(resolved)
	return ok && n > 0, resolved, nil
}

// shouldEmitUSU checks whether USU should be emitted and resolves
// its placeholder value.
func (e *Engine) shouldEmitUSU(
	placeholder string,
	values map[string]any,
	reqType uint32,
) (bool, any, error) {
	if placeholder == "" {
		return false, nil, nil
	}
	switch reqType {
	case ccReqTypeInitial:
		// INITIAL never includes USU
		return false, nil, nil
	case ccReqTypeUpdate, ccReqTypeTerminate:
		// UPDATE and TERMINATE always include USU
		resolved, err := e.resolveValue(placeholder, values)
		if err != nil {
			return false, nil, err
		}
		return true, resolved, nil
	case ccReqTypeEvent:
		// EVENT includes USU if resolved > 0
		resolved, err := e.resolveValue(placeholder, values)
		if err != nil {
			return false, nil, err
		}
		n, ok := toUint64(resolved)
		return ok && n > 0, resolved, nil
	default:
		// Unknown type — include if non-empty
		resolved, err := e.resolveValue(placeholder, values)
		if err != nil {
			return false, nil, err
		}
		return true, resolved, nil
	}
}

// buildServiceUnitAVP constructs a Requested-Service-Unit or
// Used-Service-Unit grouped AVP with the inner CC-* AVP per unitType.
func buildServiceUnitAVP(code uint32, unitType UnitType, quantity any) (*diam.AVP, error) {
	inner, err := buildInnerServiceUnitAVP(unitType, quantity)
	if err != nil {
		return nil, err
	}
	grouped := &diam.GroupedAVP{AVP: []*diam.AVP{inner}}
	return diam.NewAVP(code, avp.Mbit, 0, grouped), nil
}

// buildInnerServiceUnitAVP returns the inner CC-* AVP (CC-Total-Octets,
// CC-Time, or CC-Service-Specific-Units) per unitType.
func buildInnerServiceUnitAVP(unitType UnitType, quantity any) (*diam.AVP, error) {
	switch unitType {
	case UnitTypeOctet:
		// CC-Total-Octets is Unsigned64
		n, ok := toUint64(quantity)
		if !ok {
			return nil, fmt.Errorf("cannot encode %v as Unsigned64 for CC-Total-Octets", quantity)
		}
		return diam.NewAVP(avp.CCTotalOctets, avp.Mbit, 0, datatype.Unsigned64(n)), nil
	case UnitTypeTime:
		// CC-Time is Unsigned32 (seconds)
		n, ok := toUint32(quantity)
		if !ok {
			return nil, fmt.Errorf("cannot encode %v as Unsigned32 for CC-Time", quantity)
		}
		return diam.NewAVP(avp.CCTime, avp.Mbit, 0, datatype.Unsigned32(n)), nil
	case UnitTypeUnits:
		// CC-Service-Specific-Units is Unsigned32
		n, ok := toUint32(quantity)
		if !ok {
			return nil, fmt.Errorf("cannot encode %v as Unsigned32 for CC-Service-Specific-Units", quantity)
		}
		return diam.NewAVP(avp.CCServiceSpecificUnits, avp.Mbit, 0, datatype.Unsigned32(n)), nil
	default:
		// Default to Unsigned64 (OCTET behaviour)
		n, ok := toUint64(quantity)
		if !ok {
			return nil, fmt.Errorf("cannot encode %v as Unsigned64 (unknown unitType %q)", quantity, unitType)
		}
		return diam.NewAVP(avp.CCTotalOctets, avp.Mbit, 0, datatype.Unsigned64(n)), nil
	}
}

// resolveValue resolves a template value string by substituting
// {{TOKEN}} placeholders from the value map.
//
// If the string is exactly "{{TOKEN}}" (a single token), the typed
// value from the map is returned directly, preserving Go types (e.g.
// uint32, time.Time) for correct Diameter encoding.
//
// If the string contains text plus placeholder tokens, all tokens are
// substituted to their string representations and the result is a
// string.
//
// Returns UNRESOLVED_PLACEHOLDER if any token has no entry in values.
func (e *Engine) resolveValue(s string, values map[string]any) (any, error) {
	// Fast path: whole string is one token — preserve the typed value.
	if m := singleTokenRe.FindStringSubmatch(s); m != nil {
		key := m[1]
		val, ok := values[key]
		if !ok {
			return nil, errUnresolvedPlaceholder(key)
		}
		return val, nil
	}

	// General path: substitute all {{TOKEN}} occurrences to string.
	var firstErr error
	result := anyTokenRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := match[2 : len(match)-2] // strip "{{" and "}}"
		val, ok := values[inner]
		if !ok {
			if firstErr == nil {
				firstErr = errUnresolvedPlaceholder(inner)
			}
			return match // leave unchanged
		}
		return fmt.Sprintf("%v", val)
	})
	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

// requestTypeFromValues reads CC_REQUEST_TYPE from the value map and
// returns the Diameter CC-Request-Type uint32 (1=INITIAL, 2=UPDATE,
// 3=TERMINATE, 4=EVENT). Returns 0 if the key is absent or cannot be
// parsed.
func requestTypeFromValues(values map[string]any) uint32 {
	v, ok := values["CC_REQUEST_TYPE"]
	if !ok {
		return 0
	}
	n, ok := toUint32(v)
	if !ok {
		return 0
	}
	return n
}

// encodeValue converts a Go value to the Diameter datatype.Type
// identified by the dictionary-reported dataType string.
// Returns ENCODING_TYPE_MISMATCH when the conversion is not possible.
func encodeValue(avpName, dataType string, value any) (datatype.Type, error) {
	switch dataType {
	case "Unsigned32":
		n, ok := toUint32(value)
		if !ok {
			return nil, errEncodingTypeMismatch(avpName, "Unsigned32", fmt.Sprintf("%v", value))
		}
		return datatype.Unsigned32(n), nil

	case "Unsigned64":
		n, ok := toUint64(value)
		if !ok {
			return nil, errEncodingTypeMismatch(avpName, "Unsigned64", fmt.Sprintf("%v", value))
		}
		return datatype.Unsigned64(n), nil

	case "Integer32":
		n, ok := toInt32(value)
		if !ok {
			return nil, errEncodingTypeMismatch(avpName, "Integer32", fmt.Sprintf("%v", value))
		}
		return datatype.Integer32(n), nil

	case "Integer64":
		n, ok := toInt64(value)
		if !ok {
			return nil, errEncodingTypeMismatch(avpName, "Integer64", fmt.Sprintf("%v", value))
		}
		return datatype.Integer64(n), nil

	case "Float32":
		switch v := value.(type) {
		case float32:
			return datatype.Float32(v), nil
		case float64:
			return datatype.Float32(float32(v)), nil
		case string:
			f, err := strconv.ParseFloat(v, 32)
			if err != nil {
				return nil, errEncodingTypeMismatch(avpName, "Float32", v)
			}
			return datatype.Float32(float32(f)), nil
		default:
			return nil, errEncodingTypeMismatch(avpName, "Float32", fmt.Sprintf("%v", value))
		}

	case "Float64":
		switch v := value.(type) {
		case float64:
			return datatype.Float64(v), nil
		case float32:
			return datatype.Float64(float64(v)), nil
		case string:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, errEncodingTypeMismatch(avpName, "Float64", v)
			}
			return datatype.Float64(f), nil
		default:
			return nil, errEncodingTypeMismatch(avpName, "Float64", fmt.Sprintf("%v", value))
		}

	case "UTF8String":
		return datatype.UTF8String(fmt.Sprintf("%v", value)), nil

	case "DiameterIdentity":
		return datatype.DiameterIdentity(fmt.Sprintf("%v", value)), nil

	case "DiameterURI":
		return datatype.DiameterURI(fmt.Sprintf("%v", value)), nil

	case "OctetString":
		switch v := value.(type) {
		case []byte:
			return datatype.OctetString(v), nil
		case string:
			return datatype.OctetString(v), nil
		default:
			return datatype.OctetString(fmt.Sprintf("%v", value)), nil
		}

	case "Enumerated":
		n, ok := toInt32(value)
		if !ok {
			return nil, errEncodingTypeMismatch(avpName, "Enumerated", fmt.Sprintf("%v", value))
		}
		return datatype.Enumerated(n), nil

	case "Time":
		switch v := value.(type) {
		case time.Time:
			return datatype.Time(v), nil
		case string:
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return nil, errEncodingTypeMismatch(avpName, "Time (RFC3339)", v)
			}
			return datatype.Time(t), nil
		default:
			return nil, errEncodingTypeMismatch(avpName, "Time", fmt.Sprintf("%v", value))
		}

	case "Address":
		s := fmt.Sprintf("%v", value)
		ip := net.ParseIP(strings.TrimSpace(s))
		if ip == nil {
			return nil, errEncodingTypeMismatch(avpName, "Address (IP)", s)
		}
		if ip4 := ip.To4(); ip4 != nil {
			return datatype.Address(ip4), nil
		}
		return datatype.Address(ip.To16()), nil

	default:
		// Unknown type — treat as UTF8String (forward-compatible fallback).
		return datatype.UTF8String(fmt.Sprintf("%v", value)), nil
	}
}

// ---- numeric coercion helpers --------------------------------------

// toUint32 attempts to coerce v to uint32. Returns (0, false) on
// failure.
func toUint32(v any) (uint32, bool) {
	switch n := v.(type) {
	case uint32:
		return n, true
	case uint64:
		return uint32(n), true
	case uint:
		return uint32(n), true
	case int32:
		if n < 0 {
			return 0, false
		}
		return uint32(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint32(n), true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint32(n), true
	case float32:
		if n < 0 {
			return 0, false
		}
		return uint32(n), true
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint32(n), true
	case string:
		u, err := strconv.ParseUint(strings.TrimSpace(n), 10, 32)
		if err != nil {
			return 0, false
		}
		return uint32(u), true
	}
	return 0, false
}

// toUint64 attempts to coerce v to uint64.
func toUint64(v any) (uint64, bool) {
	switch n := v.(type) {
	case uint64:
		return n, true
	case uint32:
		return uint64(n), true
	case uint:
		return uint64(n), true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int32:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case string:
		u, err := strconv.ParseUint(strings.TrimSpace(n), 10, 64)
		if err != nil {
			return 0, false
		}
		return u, true
	}
	return 0, false
}

// toInt32 attempts to coerce v to int32.
func toInt32(v any) (int32, bool) {
	switch n := v.(type) {
	case int32:
		return n, true
	case int:
		return int32(n), true
	case int64:
		return int32(n), true
	case uint32:
		return int32(n), true
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 32)
		if err != nil {
			return 0, false
		}
		return int32(i), true
	}
	return 0, false
}

// toInt64 attempts to coerce v to int64.
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int32:
		return int64(n), true
	case int:
		return int64(n), true
	case uint64:
		return int64(n), true
	case uint32:
		return int64(n), true
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		if err != nil {
			return 0, false
		}
		return i, true
	}
	return 0, false
}
