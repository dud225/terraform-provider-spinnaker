package attribute_plan_modifier

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

var _ tfsdk.AttributePlanModifier = (*defaultValue)(nil)

type defaultValue struct {
	DefaultValue attr.Value
}

func DefaultValue(v attr.Value) tfsdk.AttributePlanModifier {
	return &defaultValue{v}
}

func (dv *defaultValue) Description(ctx context.Context) string {
	return fmt.Sprintf("Sets the default value %q (%s) if the attribute is not set", dv.DefaultValue, dv.DefaultValue.Type(ctx))
}

func (dv *defaultValue) MarkdownDescription(ctx context.Context) string {
	return dv.Description(ctx)
}

func (dv *defaultValue) Modify(_ context.Context, req tfsdk.ModifyAttributePlanRequest, res *tfsdk.ModifyAttributePlanResponse) {
	// If the attribute configuration is not null, we are done here
	if !req.AttributeConfig.IsNull() {
		return
	}

	// If the attribute plan is "known" and "not null", then a previous plan modifier in the sequence
	// has already been applied, and we don't want to interfere.
	if !req.AttributePlan.IsUnknown() && !req.AttributePlan.IsNull() {
		return
	}

	res.AttributePlan = dv.DefaultValue
}
