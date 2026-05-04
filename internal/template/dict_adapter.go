package template

import (
	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// DictAdapter wraps a *dict.Parser and implements the Dictionary interface
// required by the template engine. Use NewDictAdapter to construct one.
type DictAdapter struct {
	parser *dict.Parser
}

// NewDictAdapter wraps parser in a DictAdapter that satisfies Dictionary.
func NewDictAdapter(parser *dict.Parser) *DictAdapter {
	return &DictAdapter{parser: parser}
}

// Lookup implements Dictionary. It searches all loaded diameter applications
// for an AVP with the given name and returns its metadata.
func (a *DictAdapter) Lookup(name string) (AVPMetadata, error) {
	// ScanAVP searches all applications for a matching name or code.
	avpDef, err := a.parser.ScanAVP(name)
	if err != nil {
		return AVPMetadata{}, errUnknownAVP(name)
	}
	typeName := avpDef.Data.TypeName
	return AVPMetadata{
		Code:     avpDef.Code,
		VendorID: avpDef.VendorID,
		DataType: typeName,
		Grouped:  typeName == "Grouped",
	}, nil
}
