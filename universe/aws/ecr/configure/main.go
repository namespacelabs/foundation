// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"

	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution/defs"
	"namespacelabs.dev/foundation/universe/aws/eks"
	fniam "namespacelabs.dev/foundation/universe/aws/iam"
)

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configuration{})
	provisioning.Handle(h)
}

type configuration struct{}

func (configuration) Apply(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	eksDetails := &eks.EKSServerDetails{}
	if ok, err := req.CheckUnpackInput(eksDetails); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_examples_s3_rw-bucket.html
	policy := fniam.PolicyDocument{
		Version: "2012-10-17",
		Statement: []fniam.StatementEntry{
			{
				Effect:   "Allow",
				Action:   []string{"ecr:*"},
				Resource: []string{"*"},
			},
		},
	}

	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return fnerrors.InternalError("failed to serialize policy: %w", err)
	}

	associate := &fniam.OpAssociatePolicy{
		RoleName:   eksDetails.ComputedIamRoleName,
		PolicyName: "ns-aws-ecr-access",
		PolicyJson: string(policyBytes),
	}

	out.Invocations = append(out.Invocations, defs.Static("ECR Access IAM Policy", associate))

	return nil
}

func (configuration) Delete(context.Context, provisioning.StackRequest, *provisioning.DeleteOutput) error {
	// XXX unimplemented
	return nil
}
