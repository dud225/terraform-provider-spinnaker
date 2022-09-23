package attribute_validator

import (
  "context"
  "fmt"
  "net/mail"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type EmailValidator struct{}

func (e EmailValidator) Description(ctx context.Context) string {
  return fmt.Sprintf("Check that the email address conforms to the RFC 5322")
}

func (e EmailValidator) MarkdownDescription(ctx context.Context) string {
  return e.Description(ctx)
}

func (e EmailValidator) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
  if req.AttributeConfig.IsUnknown() || req.AttributeConfig.IsNull() {
    return
  }

  var email types.String
  resp.Diagnostics.Append(tfsdk.ValueAs(ctx, req.AttributeConfig, &email)...)
	if resp.Diagnostics.HasError() {
    return
  }

  if _, err := mail.ParseAddress(email.Value); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.AttributePath,
			"Invalid email address",
			"The email address shall conform to the RFC 5322",
		)
		return
	}
}
