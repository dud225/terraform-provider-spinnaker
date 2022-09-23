package provider

import (
  "context"
  "fmt"
  "reflect"
	"regexp"
  "strings"

  "github.com/dud225/terraform-provider-spinnaker/internal/api"
  "github.com/dud225/terraform-provider-spinnaker/internal/provider/attribute_validator"

  "github.com/mitchellh/mapstructure"

  "github.com/hashicorp/terraform-plugin-framework/attr"
  "github.com/hashicorp/terraform-plugin-framework/diag"
  "github.com/hashicorp/terraform-plugin-framework/path"
  "github.com/hashicorp/terraform-plugin-framework/resource"
  "github.com/hashicorp/terraform-plugin-framework/tfsdk"
  "github.com/hashicorp/terraform-plugin-framework/types"
  "github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
  "github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
  "github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
  "github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
  "github.com/hashicorp/terraform-plugin-log/tflog"

  "github.com/spinnaker/spin/cmd/gateclient"
)

var _ resource.ResourceWithConfigure = (*applicationResource)(nil)
var _ resource.ResourceWithImportState = (*applicationResource)(nil)

type applicationResource struct {
  api.Spinnaker
}

type application struct {
  CloudProviders  types.List          `tfsdk:"cloud_providers"`
  Email           types.String        `tfsdk:"email"`
  Id              types.String        `tfsdk:"id"`
  Name            types.String        `tfsdk:"name"`
  Permissions     map[string][]string `tfsdk:"permissions"`
}

func newApplicationResource() resource.Resource {
  return &applicationResource{}
}

func (r *applicationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
  resp.TypeName = req.ProviderTypeName + "_application"
}

func (r *applicationResource) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
  return tfsdk.Schema{
    Description: "Manage Spinnaker application",

    Attributes: map[string]tfsdk.Attribute{
      "cloud_providers": {
        Computed:       true,
        Description:    "List of the cloud providers used by the application",
        Optional:       true,
        Type:           types.ListType{
          ElemType: types.StringType,
        },
        Validators:     []tfsdk.AttributeValidator{
          // See supported cloud providers:
          // https://github.com/spinnaker/orca/blob/df21b4ceb99b109333d69d7bbf0907731301147d/orca-applications/src/main/groovy/com/netflix/spinnaker/orca/applications/utils/ApplicationNameValidator.groovy#L26-L35
          listvalidator.ValuesAre(stringvalidator.OneOf([]string{"appengine", "aws", "dcos", "gce", "kubernetes", "openstack", "titus", "tencentcloud"}...)),
        },
      },
      "email": {
        Description:    "Application owner's email",
        Required:       true,
        Type:           types.StringType,
        Validators:     []tfsdk.AttributeValidator{
          attribute_validator.EmailValidator{},
        },
      },
      "id": {
        Computed:       true,
        Description:    "Application name",
        PlanModifiers:  tfsdk.AttributePlanModifiers{
          resource.UseStateForUnknown(),
        },
        Type:           types.StringType,
      },
      "name": {
        Description:    "Application name",
        Required:       true,
				// Spinnaker doesn't generate a unique ID for an application
        PlanModifiers:  tfsdk.AttributePlanModifiers{
          resource.RequiresReplace(),
        },
        Type:           types.StringType,
				/*
					Spinnaker always store the application in lowercase
					Terraform doesn't allow a plan modifier to tweak a required value
					so to match Spinnaker's behaviour this attribute shall be set in lowercase
					otherwise the application will be constantly recreated due to the plan modifier
				*/
        Validators:     []tfsdk.AttributeValidator{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[[:lower:][:digit:]_.-]+$`), "shall be set in lowercase"),
        },
      },
      "permissions": {
        Description:    "Permissions to set on the application. It should be set as a map where the key indicates the role and the value the permissions to grant",
        Optional:       true,
        Type:           types.MapType{
          ElemType: types.SetType{
            ElemType: types.StringType,
          },
        },
        Validators:     []tfsdk.AttributeValidator{
          mapvalidator.ValuesAre(setvalidator.ValuesAre(stringvalidator.OneOf([]string{"READ", "WRITE", "EXECUTE"}...))),
        },
      },
    },
  }, nil
}

func (r *applicationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
  if req.ProviderData == nil {
    return
  }
  client, ok := req.ProviderData.(*gateclient.GatewayClient)
  if !ok {
    resp.Diagnostics.AddError(
      "Unexpected provider data type",
      fmt.Sprintf("Expected *gateclient.GatewayClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
    )
    return
  }
  r.Client = client
}

func (r *applicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
  var data application
  resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  var cloudProviders []string
  // Computed attributes, whether optional or not, will never be null in the plan for Create, Read, Update, or Delete methods.
  if data.CloudProviders.Unknown {
    data.CloudProviders = types.List{
      ElemType: types.StringType,
    }
  } else {
    resp.Diagnostics.Append(data.CloudProviders.ElementsAs(ctx, &cloudProviders, false)...)
    if resp.Diagnostics.HasError() {
      return
    }
  }

  // Optional attributes that are not computed will never be unknown in Create, Read, Update, or Delete methods.
  permissions, err := buildSpinnakerPermissions(data.Permissions)
  if err != nil {
    resp.Diagnostics.AddAttributeError(
      path.Root("permissions"),
      "Failed to format the permissions",
      "Failed to format the permissions: " + err.Error(),
    )
    return
  }

  application := map[string]interface{}{
    "cloudProviders": strings.Join(cloudProviders, ","),
    "name":           data.Name.Value,
    "email":          data.Email.Value,
  }
  if len(permissions) != 0 {
    application["permissions"] = permissions
  }

  tflog.Trace(ctx, "Creating an application", map[string]interface{}{
    "payload": application,
  })
  if err := r.CreateApplication(ctx, application); err != nil {
     resp.Diagnostics.AddError(
       "Error creating an application",
       "Failed to create an application: " + err.Error(),
     )
     return
  }

  data.Id = types.String{Value: data.Name.Value}
  resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *applicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
  var data application
  resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  res, err := r.GetApplication(ctx, data.Id.Value)
  if err != nil {
    resp.Diagnostics.AddError(
      "Error retrieving application information",
      "Failed to retrieve the application information: " + err.Error(),
    )
    return
  }
  tflog.Trace(ctx, "Retrieved application information", map[string]interface{}{
    "payload": res,
  })

  app := application{Id: data.Id}
  msd, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
    DecodeHook: applicationDecodeHookFunc(ctx),
    Result:     &app,
  })
  if err != nil {
    resp.Diagnostics.AddError(
      "Failed to create a mapstructure decoder",
      "Failed to create a mapstructure decoder: " + err.Error(),
    )
    return
  }

  if err := msd.Decode(res); err != nil {
    resp.Diagnostics.AddError(
      "Failed to decode the data",
      "Failed to decode the data received from Spinnaker: " + err.Error(),
    )
    return
  }

  resp.Diagnostics.Append(resp.State.Set(ctx, &app)...)
}

func (r applicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
  var data application
  resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  var cloudProviders []string
  if data.CloudProviders.Unknown {
    data.CloudProviders = types.List{
      ElemType: types.StringType,
    }
  } else {
    resp.Diagnostics.Append(data.CloudProviders.ElementsAs(ctx, &cloudProviders, false)...)
    if resp.Diagnostics.HasError() {
      return
    }
  }

  permissions, err := buildSpinnakerPermissions(data.Permissions)
  if err != nil {
    resp.Diagnostics.AddAttributeError(
      path.Root("permissions"),
      "Failed to format the permissions",
      "Failed to format the permissions: " + err.Error(),
    )
    return
  }

  application := map[string]interface{}{
    "cloudProviders": strings.Join(cloudProviders, ","),
    "name":           data.Name.Value,
    "email":          data.Email.Value,
  }
  if len(permissions) != 0 {
    application["permissions"] = permissions
  }

  tflog.Trace(ctx, "Updating an application", map[string]interface{}{
    "payload": application,
  })
  if err := r.CreateApplication(ctx, application); err != nil {
     resp.Diagnostics.AddError(
       "Error updating an application",
       "Failed to update an application: " + err.Error(),
     )
     return
  }

  resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *applicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
  var data application
  resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
  if resp.Diagnostics.HasError() {
    return
  }

  tflog.Trace(ctx, "Deleting an application", map[string]interface{}{
    "applicationId": data.Id.Value,
  })
  if err := r.DeleteApplication(ctx, data.Id.Value); err != nil {
    resp.Diagnostics.AddError(
      "Error deleting application",
      "Failed to delete the application: " + err.Error(),
    )
    return
  }
}

func (r *applicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
  resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applicationDecodeHookFunc(ctx context.Context) mapstructure.DecodeHookFuncType {
  return mapstructure.DecodeHookFuncType(func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
    tflog.Trace(ctx, "Executing function applicationDecodeHookFunc", map[string]interface{}{
      "from": from.String(),
      "to": to.String(),
      "data": data,
    })

    // Permissions field
    if to == reflect.TypeOf(map[string][]string(nil)) {
      permissions, ok := data.(map[string]interface{})
      if !ok {
        return nil, fmt.Errorf("applicationDecodeHookFunc: unexpected data received: %s (type: %T)", data, data)
      }
      return buildTFpermissions(permissions)
      
    // CloudProviders field
    } else if to == reflect.TypeOf(types.List{}) {
      dataStr, ok := data.(string)
      if !ok {
        return nil, fmt.Errorf("applicationDecodeHookFunc: unexpected data received: %s (type: %T)", data, data)
      }
      cloudProvidersElems := []attr.Value{}
      for _, cloudProvider := range strings.Split(dataStr, ",") {
        cloudProvidersElems = append(cloudProvidersElems, types.String{Value: cloudProvider})
      }
      return types.List{
        ElemType: types.StringType,
        Elems: cloudProvidersElems,
      }, nil

    // All other fields
    } else if to == reflect.TypeOf(types.String{}) {
      dataStr, ok := data.(string)
      if !ok {
        return nil, fmt.Errorf("applicationDecodeHookFunc: unexpected data received: %s (type: %T)", data, data)
      }
      return types.String{Value: dataStr}, nil
    }

    return data, nil
  })
}

func buildSpinnakerPermissions(TFpermissions map[string][]string) (map[string][]string, error) {
  permissions := make(map[string][]string)
  for role, perms := range TFpermissions {
    for _, perm := range perms {
      if !(perm == "READ" || perm == "WRITE" || perm == "EXECUTE") {
        return nil, fmt.Errorf("buildSpinnakerPermissions: unexpected data received: %s", perm)
      }
      if len(permissions[perm]) == 0 {
        permissions[perm] = []string{role}
      } else {
        permissions[perm] = append(permissions[perm], role)
      }
    }
  }
  return permissions, nil
}

func buildTFpermissions(SpinnakerPermissions map[string]interface{}) (map[string][]string, error) {
  permissions := make(map[string][]string)
  for perm, rolesIface := range SpinnakerPermissions {
    if !(perm == "READ" || perm == "WRITE" || perm == "EXECUTE") {
      return nil, fmt.Errorf("buildTFpermissions: unexpected data received: %s", perm)
    }
    roles, ok := rolesIface.([]interface{})
    if !ok {
      return nil, fmt.Errorf("buildTFpermissions: unexpected data received: %s (type: %T)", rolesIface, rolesIface)
    }
    for _, roleIface := range roles {
      role, ok := roleIface.(string)
      if !ok {
        return nil, fmt.Errorf("buildTFpermissions: unexpected data received: %s (type: %T)", roleIface, roleIface)
      }
      if len(permissions[role]) == 0 {
        permissions[role] = []string{perm}
      } else {
        // Duplicate permissions are possible
        var dup_perm bool
        for _, p := range permissions[role] {
          if perm == p {
            dup_perm = true
            break
          }
        }
        if !dup_perm {
          permissions[role] = append(permissions[role], perm)
        }
      }
    }
  }
  return permissions, nil
}
