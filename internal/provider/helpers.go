package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

// setString writes v into target only if v is not the zero value.
// Leaves the attribute null otherwise so Terraform doesn't see a
// computed diff against the user's omitted-attribute plan.
func setString(target *types.String, v string) {
	if v == "" {
		*target = types.StringNull()
		return
	}
	*target = types.StringValue(v)
}

// setInt is the int64 counterpart of setString.
func setInt(target *types.Int64, v int64) {
	*target = types.Int64Value(v)
}
