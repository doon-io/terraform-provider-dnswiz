package provider

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// encodeRecordData turns the type plus per-type Terraform attributes
// into the JSON envelope the dnswiz API expects on the `data` field.
//
// The shape is dictated by the server, see
// packages/server/internal/records/records.go in the dnswiz repo for
// the canonical validators. Notably SRV and CAA expect a single
// `value` string composed of space-separated fields (RFC 2782 and
// RFC 8659 respectively), not separate JSON keys per field.
func encodeRecordData(rtype string, m recordResourceModel) (json.RawMessage, error) {
	switch rtype {
	case "A", "AAAA", "CNAME", "NS", "PTR", "TXT":
		if m.Value.IsNull() || m.Value.IsUnknown() {
			return nil, fmt.Errorf("%s records require value", rtype)
		}
		return json.Marshal(map[string]any{"value": m.Value.ValueString()})
	case "ANAME":
		// Apex CNAME-flattening: server keys the envelope on "target",
		// not "value", and uses it to resolve A/AAAA at answer time.
		if m.Value.IsNull() || m.Value.IsUnknown() {
			return nil, fmt.Errorf("ANAME records require value")
		}
		return json.Marshal(map[string]any{"target": m.Value.ValueString()})
	case "MX":
		if m.Value.IsNull() || m.Value.IsUnknown() || m.Priority.IsNull() || m.Priority.IsUnknown() {
			return nil, fmt.Errorf("MX records require value and priority")
		}
		return json.Marshal(map[string]any{
			"value":    m.Value.ValueString(),
			"priority": m.Priority.ValueInt64(),
		})
	case "SRV":
		if m.Value.IsNull() || m.Priority.IsNull() || m.Weight.IsNull() || m.Port.IsNull() {
			return nil, fmt.Errorf("SRV records require value (target), priority, weight, and port")
		}
		// RFC 2782 wire format: "<priority> <weight> <port> <target>".
		v := fmt.Sprintf("%d %d %d %s",
			m.Priority.ValueInt64(),
			m.Weight.ValueInt64(),
			m.Port.ValueInt64(),
			m.Value.ValueString(),
		)
		return json.Marshal(map[string]any{"value": v})
	case "CAA":
		if m.Tag.IsNull() || m.Value.IsNull() {
			return nil, fmt.Errorf("CAA records require tag and value")
		}
		flag := int64(0)
		if !m.Flags.IsNull() && !m.Flags.IsUnknown() {
			flag = m.Flags.ValueInt64()
		}
		// RFC 8659 zonefile syntax: "<flag> <tag> <value>".
		v := fmt.Sprintf("%d %s %s", flag, m.Tag.ValueString(), m.Value.ValueString())
		return json.Marshal(map[string]any{"value": v})
	case "POOL":
		if m.PoolID.IsNull() || m.PoolID.IsUnknown() {
			return nil, fmt.Errorf("POOL records require pool_id")
		}
		return json.Marshal(map[string]any{"pool_id": m.PoolID.ValueString()})
	default:
		return nil, fmt.Errorf("unsupported record type %q (supported: A, AAAA, CNAME, NS, PTR, TXT, ANAME, MX, SRV, CAA, POOL)", rtype)
	}
}

// decodeRecordData sets the type-specific attributes on m from the
// API's data envelope. Callers should null out every type-specific
// attribute on m before calling this so that fields irrelevant to
// the record's type stay null in state.
func decodeRecordData(rtype string, data json.RawMessage, m *recordResourceModel) error {
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	str := func(k string) (string, bool) {
		v, ok := env[k].(string)
		return v, ok
	}
	num := func(k string) (int64, bool) {
		v, ok := env[k].(float64)
		return int64(v), ok
	}

	switch rtype {
	case "A", "AAAA", "CNAME", "NS", "PTR", "TXT":
		if v, ok := str("value"); ok {
			m.Value = types.StringValue(v)
		}
	case "ANAME":
		if v, ok := str("target"); ok {
			m.Value = types.StringValue(v)
		}
	case "MX":
		if v, ok := str("value"); ok {
			m.Value = types.StringValue(v)
		}
		if v, ok := num("priority"); ok {
			m.Priority = types.Int64Value(v)
		}
	case "SRV":
		// Parse the composed RFC 2782 value back into the four
		// attributes so the user's plan round-trips cleanly.
		if v, ok := str("value"); ok {
			parts := strings.Fields(v)
			if len(parts) == 4 {
				if p, err := strconv.Atoi(parts[0]); err == nil {
					m.Priority = types.Int64Value(int64(p))
				}
				if w, err := strconv.Atoi(parts[1]); err == nil {
					m.Weight = types.Int64Value(int64(w))
				}
				if port, err := strconv.Atoi(parts[2]); err == nil {
					m.Port = types.Int64Value(int64(port))
				}
				m.Value = types.StringValue(parts[3])
			}
		}
	case "CAA":
		// Parse "<flag> <tag> <value>" back into attributes.
		if v, ok := str("value"); ok {
			parts := strings.SplitN(v, " ", 3)
			if len(parts) == 3 {
				if f, err := strconv.Atoi(parts[0]); err == nil {
					m.Flags = types.Int64Value(int64(f))
				}
				m.Tag = types.StringValue(parts[1])
				m.Value = types.StringValue(strings.Trim(parts[2], `"`))
			}
		}
	case "POOL":
		if v, ok := str("pool_id"); ok {
			m.PoolID = types.StringValue(v)
		}
	}
	return nil
}
