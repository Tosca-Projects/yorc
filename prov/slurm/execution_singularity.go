// Copyright 2018 Bull S.A.S. Atos Technologies - Bull, Rue Jean Jaures, B.P.68, 78340, Les Clayes-sous-Bois, France.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slurm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/pkg/errors"

	"github.com/ystia/yorc/v3/deployments"
	"github.com/ystia/yorc/v3/events"
	"github.com/ystia/yorc/v3/log"
	"github.com/ystia/yorc/v3/tasks"
	"github.com/ystia/yorc/v3/tosca"
)

type executionSingularity struct {
	*executionCommon
	singularityInfo *singularityInfo
}

func (e *executionSingularity) execute(ctx context.Context) error {
	// Only runnable operation is currently supported
	log.Debugf("Execute the operation:%+v", e.operation)
	// Fill log optional fields for log registration
	switch strings.ToLower(e.operation.Name) {
	case strings.ToLower(tosca.RunnableSubmitOperationName):

		log.Printf("Running the job: %s", e.operation.Name)
		// Build Job Information
		if err := e.buildJobInfo(ctx); err != nil {
			return errors.Wrap(err, "failed to build job information")
		}

		// Build singularity information
		if err := e.buildSingularityInfo(ctx); err != nil {
			return errors.Wrap(err, "failed to build singularity information")
		}

		// Run the command
		err := e.prepareAndRunSingularityJob(ctx)
		if err != nil {
			events.WithContextOptionalFields(ctx).NewLogEntry(events.LogLevelERROR, e.deploymentID).RegisterAsString(err.Error())
			return errors.Wrap(err, "failed to run command")
		}

		jobInfoJSON, err := json.Marshal(e.jobInfo)
		if err != nil {
			return errors.Wrap(err, "Failed to marshal Slurm job information")
		}
		err = tasks.SetTaskData(e.kv, e.taskID, e.NodeName+"-jobInfo", string(jobInfoJSON))
		if err != nil {
			return err
		}
		// Set the JobID attribute
		// TODO(should be contextual to the current workflow)
		err = deployments.SetAttributeForAllInstances(e.kv, e.deploymentID, e.NodeName, "job_id", e.jobInfo.ID)
		if err != nil {
			return errors.Wrap(err, "failed to retrieve job id an manual cleanup may be necessary: ")
		}
	case strings.ToLower(tosca.RunnableCancelOperationName):
		jobInfo, err := e.getJobInfoFromTaskContext()
		if err != nil {
			return err
		}
		return cancelJobID(jobInfo.ID, e.client)
	default:
		return errors.Errorf("Unsupported operation %q", e.operation.Name)
	}
	return nil
}

func (e *executionSingularity) prepareAndRunSingularityJob(ctx context.Context) error {
	opts := e.fillJobOpts()
	exports := e.buildExportVars()
	innerCmd := fmt.Sprintf("%ssrun singularity %s %s %s", exports, e.singularityInfo.command, e.singularityInfo.imageURI, e.singularityInfo.exec)
	cmd := fmt.Sprintf("mkdir -p %s;sbatch -D %s %s --wrap=\"%s\"", e.jobInfo.WorkingDir, e.jobInfo.WorkingDir, opts, innerCmd)
	return e.runJob(ctx, cmd)
}

func (e *executionSingularity) buildSingularityInfo(ctx context.Context) error {
	singularityInfo := singularityInfo{}
	for _, input := range e.EnvInputs {
		if input.Name == "exec_command" && input.Value != "" {
			singularityInfo.exec = input.Value
			singularityInfo.command = "exec"
		}
	}

	singularityInfo.imageName = e.Primary
	if singularityInfo.imageName == "" {
		return errors.New("The image name is mandatory and must be filled in the operation artifact implementation")
	}

	// Default singularity command is "run"
	if singularityInfo.command == "" {
		singularityInfo.command = "run"
	}
	log.Debugf("singularity Info:%+v", singularityInfo)
	e.singularityInfo = &singularityInfo
	return e.resolveContainerImage()
}

func (e *executionSingularity) resolveContainerImage() error {
	switch {
	// Docker image
	case strings.HasPrefix(e.singularityInfo.imageName, "docker://"):
		if err := e.buildImageURI("docker://"); err != nil {
			return err
		}
		// Singularity image
	case strings.HasPrefix(e.singularityInfo.imageName, "shub://"):
		if err := e.buildImageURI("shub://"); err != nil {
			return err
		}
		// File image
	case strings.HasSuffix(e.singularityInfo.imageName, ".simg") || strings.HasSuffix(e.singularityInfo.imageName, ".img"):
		e.singularityInfo.imageURI = e.singularityInfo.imageName
	default:
		return errors.Errorf("Unable to resolve container image URI from image name:%q", e.singularityInfo.imageName)
	}
	return nil
}

func (e *executionSingularity) buildImageURI(prefix string) error {
	repoName, err := deployments.GetOperationImplementationRepository(e.kv, e.deploymentID, e.operation.ImplementedInNodeTemplate, e.NodeType, e.operation.Name)
	if err != nil {
		return err
	}
	if repoName == "" {
		e.singularityInfo.imageURI = e.singularityInfo.imageName
	} else {
		repoURL, err := deployments.GetRepositoryURLFromName(e.kv, e.deploymentID, repoName)
		if err != nil {
			return err
		}
		// Just ignore default public Docker and Singularity registries
		if repoURL == deployments.DockerHubURL || repoURL == deployments.SingularityHubURL {
			e.singularityInfo.imageURI = e.singularityInfo.imageName
		} else if repoURL != "" {
			urlStruct, err := url.Parse(repoURL)
			if err != nil {
				return err
			}
			tabs := strings.Split(e.singularityInfo.imageName, prefix)
			imageURI := prefix + path.Join(urlStruct.Host, tabs[1])
			log.Debugf("imageURI:%q", imageURI)
			e.singularityInfo.imageURI = imageURI
		} else {
			e.singularityInfo.imageURI = e.singularityInfo.imageName
		}
	}
	return nil
}
