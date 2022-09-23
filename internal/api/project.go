package api

import (
  "context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"

  "github.com/spinnaker/spin/cmd/orca-tasks"
)

func (api Spinnaker) CreateProject(ctx context.Context, project map[string]interface{}) error {
  var taskDescription, tflogMsg string
  if project["id"] == nil {
    taskDescription = fmt.Sprintf("Create project %s", project["name"])
    tflogMsg = "Submitting a Spinnaker task to request the creation of a project"
  } else {
    taskDescription = fmt.Sprintf("Update project %s", project["name"])
    tflogMsg = "Submitting a Spinnaker task to request the update of a project"
  }
	projectTask := map[string]interface{}{
		"job":         []interface{}{
      map[string]interface{}{
        "type":     "upsertProject",
        "project":  project,
        "user":     project["email"],
      },
    },
		"application": project["name"],
		"description": taskDescription,
	}

	tflog.Trace(ctx, tflogMsg, map[string]interface{}{
		"payload": projectTask,
	})
	ref, _, err := api.Client.TaskControllerApi.TaskUsingPOST1(api.Client.Context, projectTask)
  if err != nil {
		 return err
  }

	err = orca_tasks.WaitForSuccessfulTask(api.Client, ref, 5)
	if err != nil {
		return err
	}

	return nil
}

// Actually Spinnaker also allows the project name in place of the ID
func (api Spinnaker) GetProject(ctx context.Context, projectId string) (map[string]interface{}, error) {
  if projectId == "" {
    return nil, fmt.Errorf("Api.GetProject: projectId is empty")
  }

  data, resp, err := api.Client.ProjectControllerApi.GetUsingGET1(api.Client.Context, projectId)
  if resp != nil && resp.StatusCode == http.StatusNotFound {
    return nil, fmt.Errorf("Project '%s' not found", projectId)
  }
  if err != nil {
    return nil, err
  }
  if resp.StatusCode != http.StatusOK {
    return nil, fmt.Errorf("Encountered an error deleting project, status code: %d", resp.StatusCode)
  }

  return data, nil
}

func (api Spinnaker) DeleteProject(ctx context.Context, projectId string) error {
  if projectId == "" {
    return fmt.Errorf("Api.DeleteProject: projectId is empty")
  }
  if _, err := api.GetProject(ctx, projectId); err != nil {
		return err
	}

  deleteProjectTask := map[string]interface{}{
    "job":         []interface{}{
			map[string]interface{}{
				"type": "deleteProject",
				"project": map[string]interface{}{
					"id":  projectId,
				},
			},
    },
    "application": projectId,
    "description": fmt.Sprintf("Delete project %s", projectId),
  }

	tflog.Trace(ctx, "Submitting a Spinnaker task to request the deletion of a project", map[string]interface{}{
		"payload": deleteProjectTask,
	})
  ref, resp, err := api.Client.TaskControllerApi.TaskUsingPOST1(api.Client.Context, deleteProjectTask)
  if err != nil {
    return err
  }
  if resp.StatusCode != http.StatusOK {
    return fmt.Errorf("Encountered an error deleting project, status code: %d", resp.StatusCode)
  }

  err = orca_tasks.WaitForSuccessfulTask(api.Client, ref, 5)
  if err != nil {
    return err
  }

	return nil
}
