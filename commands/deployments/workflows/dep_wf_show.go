package workflows

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ystia/yorc/rest"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/ystia/yorc/commands/httputil"
)

func init() {
	var workflowName string
	var wfShowCmd = &cobra.Command{
		Use:     "show <id>",
		Short:   "Show a human readable textual representation of a given TOSCA workflow.",
		Aliases: []string{"display", "sh", "disp"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.Errorf("Expecting an id (got %d parameters)", len(args))
			}
			client, err := httputil.GetClient()
			if err != nil {
				httputil.ErrExit(err)
			}
			if workflowName == "" {
				return errors.New("Missing mandatory \"workflow-name\" parameter")
			}
			request, err := client.NewRequest("GET", fmt.Sprintf("/deployments/%s/workflows/%s", args[0], workflowName), nil)
			if err != nil {
				httputil.ErrExit(err)
			}
			request.Header.Add("Accept", "application/json")
			response, err := client.Do(request)
			defer response.Body.Close()
			if err != nil {
				httputil.ErrExit(err)
			}
			ids := args[0] + "/" + workflowName
			httputil.HandleHTTPStatusCode(response, ids, "deployment/workflow", http.StatusOK)

			var wf rest.Workflow
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				httputil.ErrExit(err)
			}
			err = json.Unmarshal(body, &wf)
			if err != nil {
				httputil.ErrExit(err)
			}
			fmt.Printf("Workflow %s:\n", workflowName)
			for stepName, step := range wf.Steps {
				fmt.Printf("  Step %s:\n", stepName)
				if step.Target != "" {
					fmt.Println("    Target:", step.Target)
				}
				fmt.Println("    Activities:")
				for _, activity := range step.Activities {
					if activity.CallOperation != "" {
						fmt.Println("      - Call Operation:", activity.CallOperation)
					}
					if activity.Delegate != "" {
						fmt.Println("      - Delegate:", activity.Delegate)
					}
					if activity.SetState != "" {
						fmt.Println("      - Set State:", activity.SetState)
					}
					if activity.Inline != "" {
						fmt.Println("      - Inline:", activity.Inline)
					}
				}
				if len(step.OnSuccess) > 0 {
					fmt.Println("    On Success:")
					for _, next := range step.OnSuccess {
						fmt.Println("      -", next)
					}
				}
			}
			return nil
		},
	}
	wfShowCmd.PersistentFlags().StringVarP(&workflowName, "workflow-name", "w", "", "The workflows name")
	workflowsCmd.AddCommand(wfShowCmd)
}