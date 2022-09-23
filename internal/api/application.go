package api

import (
  "context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"

  "github.com/spinnaker/spin/cmd/orca-tasks"
)

func (api Spinnaker) CreateApplication(ctx context.Context, app map[string]interface{}) error {
	createAppTask := map[string]interface{}{
		"job":         []interface{}{
      map[string]interface{}{
        "type":       "createApplication",
        "application": app,
      },
    },
		"application": app["name"],
		"description": fmt.Sprintf("Create application %s", app["name"]),
	}

	tflog.Trace(ctx, "Submitting a Spinnaker task to request the creation of an application", map[string]interface{}{
		"payload": createAppTask,
	})
	ref, _, err := api.Client.TaskControllerApi.TaskUsingPOST1(api.Client.Context, createAppTask)
  if err != nil {
		 return err
  }

	err = orca_tasks.WaitForSuccessfulTask(api.Client, ref, 5)
	if err != nil {
		return err
	}

	return nil
}

func (api Spinnaker) GetApplication(ctx context.Context, appName string) (map[string]interface{}, error) {
  if appName == "" {
    return nil, fmt.Errorf("Api.GetApplication: appName is empty")
  }

  data, resp, err := api.Client.ApplicationControllerApi.GetApplicationUsingGET(api.Client.Context, appName, nil)
  if resp != nil && resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Application '%s' not found", appName)
  }
  if err != nil {
    return nil, err
  }
  if resp.StatusCode != http.StatusOK {
    return nil, fmt.Errorf("Encountered an error deleting application, status code: %d", resp.StatusCode)
  }

  attributes, ok := data["attributes"].(map[string]interface{})
  if !ok {
    return nil, fmt.Errorf("While getting information about an application, an unexpected type (%T) was received. This is always a bug in the provider code and should be reported to the provider developers.", data["attributes"])
  }
  return attributes, nil
}

func (api Spinnaker) DeleteApplication(ctx context.Context, appName string) error {
  if appName == "" {
    return fmt.Errorf("Api.DeleteApplication: appName is empty")
  }
  if _, err := api.GetApplication(ctx, appName); err != nil {
		return err
	}

  deleteAppTask := map[string]interface{}{
    "job":         []interface{}{
			map[string]interface{}{
				"type": "deleteApplication",
				"application": map[string]interface{}{
					"name": appName,
				},
			},
    },
    "application": appName,
    "description": fmt.Sprintf("Delete application %s", appName),
  }

	tflog.Trace(ctx, "Submitting a Spinnaker task to request the deletion of an application", map[string]interface{}{
		"payload": deleteAppTask,
	})
  ref, resp, err := api.Client.TaskControllerApi.TaskUsingPOST1(api.Client.Context, deleteAppTask)
  if err != nil {
    return err
  }
  if resp.StatusCode != http.StatusOK {
    return fmt.Errorf("Encountered an error deleting application, status code: %d", resp.StatusCode)
  }

  err = orca_tasks.WaitForSuccessfulTask(api.Client, ref, 5)
  if err != nil {
    return err
  }

	return nil
}

func (api Spinnaker) ListApplications(ctx context.Context) ([]map[string]interface{}, error) {
	res, _, err := api.Client.ApplicationControllerApi.GetAllApplicationsUsingGET(api.Client.Context, nil)
  if err != nil {
    return nil, err
  }

  appList := make([]map[string]interface{}, len(res))
	for index, appInfoIface := range res {
		appInfo, ok := appInfoIface.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("While getting information about an application, an unexpected type (%T) was received. This is always a bug in the provider code and should be reported to the provider developers.", appInfoIface)
		}
		appList[index] = appInfo
	}

	return appList, nil
}
