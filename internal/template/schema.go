// JSON template body schema — the shape of the avp_template.body
// JSONB column.
//
// This shape is a contract (see decision issue #146). Any change
// to field names, removal of fields, or addition of required fields
// is a breaking change and must follow the migration procedure
// documented there.

package template

import "encoding/json"

// supportedVersion is the only template body version this package
// can parse. The loader rejects any body with a different version.
const supportedVersion = 1

// TemplateBody is the top-level JSON object stored in the
// avp_template.body JSONB column.
//
// Round-trip rule: marshalling a TemplateBody that was produced by
// Unmarshal must yield bytes that Unmarshal back to an equal value.
type TemplateBody struct {
	// Version is the schema version. Only version 1 is supported;
	// any other value causes Validate to return
	// UNKNOWN_TEMPLATE_VERSION.
	Version int `json:"version"`

	// AVPs is the recursive AVP tree. Each entry is either a leaf
	// node (Value set, no child AVPs) or a grouped node (child AVPs
	// set, no Value). Mixed nodes — both Value and child AVPs set —
	// are rejected by Validate.
	AVPs []AVPNode `json:"avps,omitempty"`

	// MSCC is the list of Multiple-Services-Credit-Control block
	// specs. The engine constructs one MSCC grouped AVP per entry.
	// Absent in templates that use the root or single-mscc service
	// model with no explicit MSCC declaration.
	MSCC []MSCCTemplateBlock `json:"mscc,omitempty"`

	// StaticValues maps placeholder token names to their default
	// values. These are the lowest-priority value source; they are
	// overridden by execution-context variables and step overrides.
	StaticValues map[string]string `json:"staticValues,omitempty"`

	// GeneratedValues lists the placeholder token names whose values
	// are auto-generated at load time by the GeneratorProvider
	// (Session-Id, Charging-Id, CC-Request-Number, timestamps, etc.).
	// A generated value is only used if the token has no static or
	// variable value — it fills the gap.
	GeneratedValues []string `json:"generatedValues,omitempty"`
}

// AVPNode is a single node in the template's AVP tree.
//
// A node is either a leaf (Value non-empty, AVPs nil or empty) or a
// grouped AVP (AVPs non-nil, Value empty). Having both Value and child
// AVPs set is invalid and causes Validate to return INVALID_AVP_NODE.
type AVPNode struct {
	// Name is the Diameter AVP name as it appears in the dictionary
	// (e.g. "Origin-Host", "Service-Information"). Required.
	Name string `json:"name"`

	// VendorID is the Vendor-Id for this AVP and is inherited by
	// descendant nodes that do not declare their own VendorID.
	// Zero means "no vendor" (IETF base AVPs).
	VendorID int64 `json:"vendorId,omitempty"`

	// Value is the leaf value. May contain {{PLACEHOLDER}} tokens
	// that the engine substitutes from the resolved value map.
	// Mutually exclusive with a non-empty AVPs list.
	Value string `json:"value,omitempty"`

	// AVPs is the list of child nodes for grouped AVPs.
	// Mutually exclusive with a non-empty Value.
	AVPs []AVPNode `json:"avps,omitempty"`
}

// MSCCTemplateBlock is one entry in the template body's mscc list.
//
// Each entry drives the construction of one
// Multiple-Services-Credit-Control grouped AVP in the engine.
// RatingGroup and ServiceIdentifier may contain {{PLACEHOLDER}}
// tokens that the engine resolves from the value map.
type MSCCTemplateBlock struct {
	// RatingGroup is the Rating-Group value (or placeholder reference).
	// Zero omits the Rating-Group AVP from the MSCC block.
	RatingGroup uint32 `json:"ratingGroup,omitempty"`

	// ServiceIdentifier is the Service-Identifier value (or
	// placeholder reference). Zero omits the AVP.
	ServiceIdentifier uint32 `json:"serviceIdentifier,omitempty"`

	// Requested is a placeholder reference resolving to the
	// Requested-Service-Unit quantity. May be empty when no RSU is
	// needed (e.g. TERMINATE requests).
	Requested string `json:"requested,omitempty"`

	// Used is a placeholder reference resolving to the
	// Used-Service-Unit quantity. May be empty when no USU is needed
	// (e.g. INITIAL requests).
	Used string `json:"used,omitempty"`
}

// Validate checks the TemplateBody for structural correctness.
//
// Returns:
//   - UNKNOWN_TEMPLATE_VERSION if Version != 1.
//   - INVALID_AVP_NODE for any AVP node that has both Value and child
//     AVPs set simultaneously.
//
// Validate does not perform dictionary lookups; unknown AVP names are
// detected later by the engine.
func (b *TemplateBody) Validate() error {
	if b.Version != supportedVersion {
		return errUnknownTemplateVersion(b.Version)
	}
	for i := range b.AVPs {
		if err := validateAVPNode(&b.AVPs[i]); err != nil {
			return err
		}
	}
	return nil
}

// validateAVPNode recursively checks that no node has both Value and
// child AVPs set.
func validateAVPNode(n *AVPNode) error {
	if n.Value != "" && len(n.AVPs) > 0 {
		return errInvalidAVPNode(n.Name,
			"node has both a value and child AVPs (mixed mode is not allowed)")
	}
	for i := range n.AVPs {
		if err := validateAVPNode(&n.AVPs[i]); err != nil {
			return err
		}
	}
	return nil
}

// ParseTemplateBody parses a raw JSON byte slice into a TemplateBody
// and validates it. It is the canonical entry point for loading a
// template body from the store; callers should prefer it over manual
// json.Unmarshal + Validate.
func ParseTemplateBody(raw []byte) (TemplateBody, error) {
	var body TemplateBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return TemplateBody{}, err
	}
	if err := body.Validate(); err != nil {
		return TemplateBody{}, err
	}
	return body, nil
}
