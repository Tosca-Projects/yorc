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

package validation

import (
	"context"

	"github.com/ystia/yorc/v4/config"
	"github.com/ystia/yorc/v4/deployments"
	"github.com/ystia/yorc/v4/events"
	"github.com/ystia/yorc/v4/tasks"
	"github.com/ystia/yorc/v4/tasks/workflow"
	"github.com/ystia/yorc/v4/tasks/workflow/builder"
)

func postComputeCreationHook(ctx context.Context, cfg config.Configuration, taskID, deploymentID, target string, activity builder.Activity) {

	if activity.Type() != builder.ActivityTypeDelegate && activity.Type() != builder.ActivityTypeCallOperation {
		return
	}
	status, err := tasks.GetTaskStatus(taskID)
	if err != nil {
		events.WithContextOptionalFields(ctx).NewLogEntry(events.LogLevelWARN, deploymentID).
			Registerf("Failed to retrieve task status when ensuring that a compute will have it's endpoint ip set. Next operations will likely fail: %v", err)
		return
	}
	if status == tasks.TaskStatusFAILED || status == tasks.TaskStatusCANCELED {
		return
	}

	isCompute, err := deployments.IsNodeDerivedFrom(ctx, deploymentID, target, "yorc.nodes.Compute")
	if err != nil {
		events.WithContextOptionalFields(ctx).NewLogEntry(events.LogLevelWARN, deploymentID).
			Registerf("Failed to retrieve node type for node %q when ensuring that a compute will have it's endpoint ip set. Next operations will likely failed: %v", target, err)
		return
	}
	if !isCompute {
		return
	}
	instances, err := deployments.GetNodeInstancesIds(ctx, deploymentID, target)
	if err != nil {
		events.WithContextOptionalFields(ctx).NewLogEntry(events.LogLevelWARN, deploymentID).
			Registerf("Failed to retrieve node instances for node %q when ensuring that a compute will have it's endpoint ip set. Next operations will likely failed: %v", target, err)
		return
	}
	checkAllInstances(ctx, deploymentID, target, instances)
}

func checkAllInstances(ctx context.Context, deploymentID, target string, instances []string) {
	for _, instance := range instances {
		ipAddress, err := deployments.GetInstanceCapabilityAttributeValue(ctx, deploymentID, target, instance, "endpoint", "ip_address")
		if err != nil {
			events.WithContextOptionalFields(ctx).NewLogEntry(events.LogLevelWARN, deploymentID).
				Registerf("Failed to retrieve node attribute for node %q when ensuring that a compute will have it's endpoint ip set. Next operations will likely failed: %v", target, err)
			return
		}
		if ipAddress == nil {
			// Check those attributes in order. Stop at the first found.
			for _, attr := range []string{"public_ip_address", "public_address", "private_address", "ip_address"} {
				found, err := setEndpointIPFromAttribute(ctx, deploymentID, target, instance, attr)
				if err != nil {
					events.WithContextOptionalFields(ctx).NewLogEntry(events.LogLevelWARN, deploymentID).
						Registerf("Failed to retrieve node attribute for node %q when ensuring that a compute will have it's endpoint ip set. Next operations will likely failed: %v", target, err)
					return
				}
				if found {
					break
				}
			}
		}
	}
}

func setEndpointIPFromAttribute(ctx context.Context, deploymentID, nodeName, instance, attribute string) (bool, error) {
	ip, err := deployments.GetInstanceAttributeValue(ctx, deploymentID, nodeName, instance, attribute)
	if err != nil {
		return false, err
	}
	if ip != nil && ip.RawString() != "" {
		err = deployments.SetInstanceCapabilityAttribute(ctx, deploymentID, nodeName, instance, "endpoint", "ip_address", ip.RawString())
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func init() {
	workflow.RegisterPostActivityHook(postComputeCreationHook)
}
