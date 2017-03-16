package commands

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"novaforge.bull.com/starlings-janus/janus/helper/ziputil"
)

func init() {
	var shouldStreamLogs bool
	var shouldStreamEvents bool
	var deployCmd = &cobra.Command{
		Use:   "deploy <csar_path>",
		Short: "Deploy a CSAR",
		Long: `Deploy a file or directory pointed by <csar_path>
	If <csar_path> point to a valid zip archive it is submitted to Janus as it.
	If <csar_path> point to a file or directory it is zipped before beeing submitted to Janus.
	If <csar_path> point to a single file it should be TOSCA YAML description.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("Expecting a path to a file or directory (got %d parameters)", len(args))
			}
			client, err := getClient()
			if err != nil {
				errExit(err)
			}
			absPath, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			fileInfo, err := os.Stat(absPath)
			if err != nil {
				return err
			}
			var location string = ""
			if !fileInfo.IsDir() {
				file, err := os.Open(absPath)
				if err != nil {
					errExit(err)
				}

				defer file.Close()

				buff, err := ioutil.ReadAll(file)
				if err != nil {
					errExit(err)
				}
				fileType := http.DetectContentType(buff)
				if fileType == "application/zip" {
					location, err = postCSAR(buff, client)
					if err != nil {
						errExit(err)
					}
				}
			}

			if location == "" {
				csarZip, err := ziputil.ZipPath(absPath)
				if err != nil {
					errExit(err)
				}
				location, err = postCSAR(csarZip, client)

				if err != nil {
					errExit(err)
				}
			}

			fmt.Println("Deployment submitted. Deployment Id:", path.Base(location))
			if shouldStreamLogs && !shouldStreamEvents {
				streamsLogs(client, path.Base(location), !noColor, true, false)
			} else if !shouldStreamLogs && shouldStreamEvents {
				streamsEvents(client, path.Base(location), !noColor, true, false)
			} else if shouldStreamLogs && shouldStreamEvents {
				return errors.Errorf("You can't provide stream-events and stream-logs flags at same time")
			}
			return nil
		},
	}
	deployCmd.PersistentFlags().BoolVarP(&shouldStreamLogs, "stream-logs", "l", false, "Stream logs after deploying the CSAR. In this mode logs can't be filtered, to use this feature see the \"log\" command.")
	deployCmd.PersistentFlags().BoolVarP(&shouldStreamEvents, "stream-events", "e", false, "Stream events after deploying the CSAR.")
	deploymentsCmd.AddCommand(deployCmd)
}

func postCSAR(csarZip []byte, client *janusClient) (string, error) {
	r, err := client.Post("/deployments", "application/zip", bytes.NewReader(csarZip))
	if err != nil {
		return "", err
	}
	if r.StatusCode != 201 {
		// Try to get the reason
		printErrors(r.Body)
		return "", fmt.Errorf("POST failed: Expecting HTTP Status code 201 got %d, reason %q", r.StatusCode, r.Status)
	}
	if location := r.Header.Get("Location"); location != "" {
		return location, nil
	}
	return "", errors.New("No \"Location\" header returned in Janus response")
}
