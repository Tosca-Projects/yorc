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

package aws

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/ystia/yorc/v4/deployments"
	"github.com/ystia/yorc/v4/prov/terraform/commons"
)

func (g *awsGenerator) generateVPC(ctx context.Context, nodeParams nodeParams) error {
	err := verifyThatNodeIsTypeOf(ctx, nodeParams, "yorc.nodes.aws.VPC")
	if err != nil {
		return err
	}

	vpc := &VPC{}

	stringParams := []struct {
		pAttr        *string
		propertyName string
		mandatory    bool
	}{
		{&vpc.CidrBlock, "cidr_block", true},
		{&vpc.InstanceTenancy, "instance_tenancy", false},
		{&vpc.EnableDNSSupport, "enable_dns_support", false},
		{&vpc.EnableDNSHostnames, "enable_dns_hostnames", false},
		{&vpc.EnableClassiclink, "enable_classiclink", false},
		{&vpc.EnableClassiclinkDNSSupport, "enable_classiclink_dns_support", false},
		{&vpc.AssignGeneratedIpv6CidrBlock, "assign_generated_ipv6_cidr_block", false},
	}

	for _, stringParam := range stringParams {
		if *stringParam.pAttr, err = deployments.GetStringNodeProperty(ctx, nodeParams.deploymentID, nodeParams.nodeName, stringParam.propertyName, stringParam.mandatory); err != nil {
			return errors.Wrapf(err, "failed to generate private network for deploymentID:%q, nodeName:%q", nodeParams.deploymentID, nodeParams.nodeName)
		}
	}

	// Get tags map
	tagsVal, err := deployments.GetNodePropertyValue(ctx, nodeParams.deploymentID, nodeParams.nodeName, "tags")
	if tagsVal != nil && tagsVal.RawString() != "" {
		d, ok := tagsVal.Value.(map[string]interface{})
		if !ok {
			return errors.New("failed to retrieve tags map from Tosca Value: not expected type")
		}

		vpc.Tags = make(map[string]string, len(d))
		for k, v := range d {
			v, ok := v.(string)
			if !ok {
				return errors.Errorf("failed to retrieve string value from tags map from Tosca Value:%q not expected type", v)
			}
			vpc.Tags[k] = v
		}
	}

	// Create the name for the resource
	var name = ""
	if vpc.Tags["Name"] != "" {
		name = vpc.Tags["Name"]
	} else {
		name = strings.ToLower(nodeParams.deploymentID + "_" + nodeParams.nodeName)
	}

	commons.AddResource(nodeParams.infrastructure, "aws_vpc", name, vpc)

	// Terraform Outputs

	return nil
}
