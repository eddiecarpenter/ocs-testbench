// Package messaging — Diameter message pretty-printer.
//
// FormatDiameterMessage renders a *diam.Message as the human-readable
// "AVP tree" format used in DRA debug logs. Ported from the charging-domain
// project's PrintDiameterMessage; adapted to return a string instead of
// writing to stdout so callers can embed the output in API responses.
package messaging

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// FormatDiameterMessage renders m as a human-readable AVP tree string.
// The first line is the message header; subsequent lines are one AVP per
// line, indented two spaces per nesting level.
func FormatDiameterMessage(m *diam.Message) string {
	if m == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Diameter Message: CommandCode: %d, appId: %d, flags: %d\n",
		m.Header.CommandCode,
		m.Header.ApplicationID,
		m.Header.CommandFlags,
	)
	dp := m.Dictionary()
	for _, a := range m.AVP {
		ppAVP(a, 0, m.Header.ApplicationID, dp, &b)
	}
	return b.String()
}

func ppAVP(a *diam.AVP, indent int, appID uint32, dp *dict.Parser, b *strings.Builder) {
	prefix := strings.Repeat("  ", indent)
	name := ppAVPName(a, appID, dp)

	if g, ok := a.Data.(*diam.GroupedAVP); ok {
		b.WriteString(ppFormatLine(prefix, a.Code, name, "<Grouped>"))
		for _, child := range g.AVP {
			ppAVP(child, indent+1, appID, dp, b)
		}
		return
	}
	if raw, ok := a.Data.(datatype.Grouped); ok {
		g, err := diam.DecodeGrouped(raw, appID, dp)
		if err != nil {
			fmt.Fprintf(b, "%s%d: %-30s <Grouped decode error: %v>\n", prefix, a.Code, name, err)
			return
		}
		b.WriteString(ppFormatLine(prefix, a.Code, name, "<Grouped>"))
		for _, child := range g.AVP {
			ppAVP(child, indent+1, appID, dp, b)
		}
		return
	}

	b.WriteString(ppFormatLine(prefix, a.Code, name, ppFormatValue(a.Data)))
}

func ppAVPName(a *diam.AVP, appID uint32, dp *dict.Parser) string {
	if dp == nil {
		return "<unknown>"
	}
	if def, err := dp.FindAVPWithVendor(appID, a.Code, a.VendorID); err == nil {
		return def.Name
	}
	if def, err := dp.FindAVPWithVendor(0, a.Code, a.VendorID); err == nil {
		return def.Name
	}
	return fmt.Sprintf("AVP(%d)", a.Code)
}

func ppFormatLine(prefix string, code uint32, name string, value string) string {
	var line strings.Builder
	line.WriteString(fmt.Sprintf("%s%d: %s", prefix, code, name))
	for line.Len() < 40 {
		if line.Len()%2 == 0 {
			line.WriteString(".")
		} else {
			line.WriteString(" ")
		}
	}
	line.WriteString(value)
	line.WriteString("\n")
	return line.String()
}

func ppFormatValue(v datatype.Type) string {
	switch t := v.(type) {
	case datatype.UTF8String:
		return string(t)
	case datatype.OctetString:
		if utf8.Valid([]byte(t)) {
			return string(t)
		}
		return fmt.Sprintf("0x%x", []byte(t))
	case datatype.DiameterIdentity:
		return string(t)
	case datatype.DiameterURI:
		return string(t)
	case datatype.Enumerated:
		return fmt.Sprintf("%d", uint32(t))
	case datatype.Unsigned32:
		return fmt.Sprintf("%d", uint32(t))
	case datatype.Integer32:
		return fmt.Sprintf("%d", int32(t))
	case datatype.Unsigned64:
		return fmt.Sprintf("%d", uint64(t))
	case datatype.Integer64:
		return fmt.Sprintf("%d", int64(t))
	case datatype.Time:
		return time.Time(t).Format(time.RFC1123Z)
	default:
		return fmt.Sprintf("%v", t)
	}
}
