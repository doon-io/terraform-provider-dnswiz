package provider

import (
	"encoding/json"
	"fmt"
)

// encodeRecordData turns the type plus per-type Terraform attributes
// into the JSON envelope the dnswiz API expects on the `data` field.
//
// The shape is dictated by the server, see
// packages/server/internal/records/records.go in the dnswiz repo for
// the canonical validators.
func encodeRecordData(rtype string, m recordResourceModel) (json.RawMessage, error) {
	switch rtype {
	case "A", "AAAA", "CNAME", "NS", "PTR", "DNAME", "TXT", "ANAME":
		if m.Value.IsNull() || m.Value.IsUnknown() {
			return nil, fmt.Errorf("%s records require value", rtype)
		}
		return json.Marshal(map[string]any{"value": m.Value.ValueString()})
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
			return nil, fmt.Errorf("SRV records require value, priority, weight, and port")
		}
		return json.Marshal(map[string]any{
			"value":    m.Value.ValueString(),
			"priority": m.Priority.ValueInt64(),
			"weight":   m.Weight.ValueInt64(),
			"port":     m.Port.ValueInt64(),
		})
	case "CAA":
		if m.Tag.IsNull() || m.Value.IsNull() {
			return nil, fmt.Errorf("CAA records require tag and value")
		}
		out := map[string]any{
			"tag":   m.Tag.ValueString(),
			"value": m.Value.ValueString(),
		}
		if !m.Flags.IsNull() && !m.Flags.IsUnknown() {
			out["flags"] = m.Flags.ValueInt64()
		}
		return json.Marshal(out)
	case "POOL":
		if m.PoolID.IsNull() || m.PoolID.IsUnknown() {
			return nil, fmt.Errorf("POOL records require pool_id")
		}
		return json.Marshal(map[string]any{"pool_id": m.PoolID.ValueString()})
	default:
		return nil, fmt.Errorf("unsupported record type %q (supported: A, AAAA, CNAME, NS, PTR, DNAME, TXT, ANAME, MX, SRV, CAA, POOL)", rtype)
	}
}

// decodeRecordData copies fields out of the API's data envelope back
// into the resource model. Fields not relevant to the type are left
// at their null/unknown state so Terraform doesn't see a diff.
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
	case "A", "AAAA", "CNAME", "NS", "PTR", "DNAME", "TXT", "ANAME":
		if v, ok := str("value"); ok {
			setString(&m.Value, v)
		}
	case "MX":
		if v, ok := str("value"); ok {
			setString(&m.Value, v)
		}
		if v, ok := num("priority"); ok {
			setInt(&m.Priority, v)
		}
	case "SRV":
		if v, ok := str("value"); ok {
			setString(&m.Value, v)
		}
		if v, ok := num("priority"); ok {
			setInt(&m.Priority, v)
		}
		if v, ok := num("weight"); ok {
			setInt(&m.Weight, v)
		}
		if v, ok := num("port"); ok {
			setInt(&m.Port, v)
		}
	case "CAA":
		if v, ok := str("tag"); ok {
			setString(&m.Tag, v)
		}
		if v, ok := str("value"); ok {
			setString(&m.Value, v)
		}
		if v, ok := num("flags"); ok {
			setInt(&m.Flags, v)
		}
	case "POOL":
		if v, ok := str("pool_id"); ok {
			setString(&m.PoolID, v)
		}
	}
	return nil
}
