package messaging

import (
	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
)

// DecodeCCAMessage projects an incoming *diam.Message onto a
// *CCA. The decoder is permissive — fields that are absent leave
// the corresponding *CCA field at its zero value; the decoder
// does not validate the message against an RFC schema (the
// dictionary already did that at parse time).
//
// FUIAction is the message-level Final-Unit-Indication's action
// when present; -1 indicates absent. MSCC blocks each carry their
// own per-MSCC FUIAction.
func DecodeCCAMessage(m *diam.Message) (*CCA, error) {
	if m == nil {
		return nil, ErrInvalidCCR // reuse — nil input is a caller bug
	}
	cca := &CCA{
		Raw:       m,
		FUIAction: -1, // sentinel: "no top-level FUI"
	}

	if a, err := m.FindAVP(avp.SessionID, 0); err == nil {
		if v, ok := a.Data.(datatype.UTF8String); ok {
			cca.SessionID = string(v)
		}
	}
	if a, err := m.FindAVP(avp.OriginHost, 0); err == nil {
		if v, ok := a.Data.(datatype.DiameterIdentity); ok {
			cca.OriginHost = string(v)
		}
	}
	if a, err := m.FindAVP(avp.OriginRealm, 0); err == nil {
		if v, ok := a.Data.(datatype.DiameterIdentity); ok {
			cca.OriginRealm = string(v)
		}
	}
	if a, err := m.FindAVP(avp.AuthApplicationID, 0); err == nil {
		if v, ok := a.Data.(datatype.Unsigned32); ok {
			cca.AuthApplicationID = uint32(v)
		}
	}
	if a, err := m.FindAVP(avp.ResultCode, 0); err == nil {
		if v, ok := a.Data.(datatype.Unsigned32); ok {
			cca.ResultCode = uint32(v)
		}
	}
	if a, err := m.FindAVP(avp.CCRequestType, 0); err == nil {
		if v, ok := a.Data.(datatype.Enumerated); ok {
			cca.CCRequestType = uint32(v)
		}
	}
	if a, err := m.FindAVP(avp.CCRequestNumber, 0); err == nil {
		if v, ok := a.Data.(datatype.Unsigned32); ok {
			cca.CCRequestNumber = uint32(v)
		}
	}
	if a, err := m.FindAVP(avp.ValidityTime, 0); err == nil {
		// Top-level Validity-Time only — the FindAVP API does
		// not walk into MSCC sub-trees by default; per-MSCC
		// Validity-Time is read out of MSCCBlock instead.
		if v, ok := a.Data.(datatype.Unsigned32); ok {
			cca.ValidityTime = uint32(v)
		}
	}
	if a, err := m.FindAVP(avp.FinalUnitIndication, 0); err == nil {
		if g, ok := a.Data.(*diam.GroupedAVP); ok {
			cca.FUIAction = readFUIAction(g)
		}
	}

	cca.MSCC = decodeMSCCs(m)
	return cca, nil
}

// readFUIAction extracts the Final-Unit-Action sub-AVP from a
// Final-Unit-Indication grouped AVP, returning -1 when absent.
func readFUIAction(g *diam.GroupedAVP) int32 {
	for _, sub := range g.AVP {
		if sub.Code == avp.FinalUnitAction {
			if v, ok := sub.Data.(datatype.Enumerated); ok {
				return int32(v)
			}
		}
	}
	return -1
}

// decodeMSCCs walks every Multiple-Services-Credit-Control AVP at
// the message level and produces an ordered slice of MSCCBlocks.
// Sub-fields are read by walking the grouped AVP children — the
// decoder does not depend on dictionary metadata for the layout
// (the structure is fixed by RFC 4006 §8.16).
func decodeMSCCs(m *diam.Message) []MSCCBlock {
	avps, err := m.FindAVPs(avp.MultipleServicesCreditControl, 0)
	if err != nil || len(avps) == 0 {
		return nil
	}
	out := make([]MSCCBlock, 0, len(avps))
	for _, mscc := range avps {
		g, ok := mscc.Data.(*diam.GroupedAVP)
		if !ok {
			continue
		}
		block := MSCCBlock{
			Raw:       mscc,
			FUIAction: -1,
		}
		for _, sub := range g.AVP {
			switch sub.Code {
			case avp.ServiceIdentifier:
				if v, ok := sub.Data.(datatype.Unsigned32); ok {
					block.ServiceIdentifier = uint32(v)
				}
			case avp.RatingGroup:
				if v, ok := sub.Data.(datatype.Unsigned32); ok {
					block.RatingGroup = uint32(v)
				}
			case avp.ResultCode:
				if v, ok := sub.Data.(datatype.Unsigned32); ok {
					block.ResultCode = uint32(v)
				}
			case avp.ValidityTime:
				if v, ok := sub.Data.(datatype.Unsigned32); ok {
					block.ValidityTime = uint32(v)
				}
			case avp.GrantedServiceUnit:
				if g, ok := sub.Data.(*diam.GroupedAVP); ok {
					block.GrantedTime, block.GrantedTotalOctets = readGrantedUnits(g)
				}
			case avp.FinalUnitIndication:
				if g, ok := sub.Data.(*diam.GroupedAVP); ok {
					block.FUIAction = readFUIAction(g)
				}
			}
		}
		out = append(out, block)
	}
	return out
}

// readGrantedUnits extracts CC-Time and CC-Total-Octets from a
// Granted-Service-Unit grouped AVP, returning zero for absent
// sub-fields.
func readGrantedUnits(g *diam.GroupedAVP) (ccTime uint32, ccTotalOctets uint64) {
	for _, sub := range g.AVP {
		switch sub.Code {
		case avp.CCTime:
			if v, ok := sub.Data.(datatype.Unsigned32); ok {
				ccTime = uint32(v)
			}
		case avp.CCTotalOctets:
			if v, ok := sub.Data.(datatype.Unsigned64); ok {
				ccTotalOctets = uint64(v)
			}
		}
	}
	return ccTime, ccTotalOctets
}
