// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package registry

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type ecrManager struct {
	sesh    aws.Config
	profile string
}

var _ registry.Manager = ecrManager{}

func Register() {
	registry.Register("aws/ecr", func(ctx context.Context, env ops.Environment) (m registry.Manager, finalErr error) {
		sesh, profile, err := awsprovider.ConfiguredSession(ctx, env.DevHost(), env.Proto())
		if err != nil {
			return nil, err
		}

		return ecrManager{sesh: sesh, profile: profile}, nil
	})
}

func (em ecrManager) client() *ecr.Client {
	return ecr.NewFromConfig(em.sesh)
}

func packageURL(repo, packageName string) string {
	return fmt.Sprintf("%s/%s", repo, packageName)
}

func repoURL(sesh aws.Config, caller *sts.GetCallerIdentityOutput) string {
	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", *caller.Account, sesh.Region)
}

func (em ecrManager) IsInsecure() bool { return false }

func (em ecrManager) Tag(ctx context.Context, packageName schema.PackageName, version provision.BuildID) (oci.AllocatedName, error) {
	res, err := compute.Get(ctx, keychainSession(em).resolveAccount())
	if err != nil {
		return oci.AllocatedName{}, err
	}

	caller := res.Value
	url := packageURL(repoURL(em.sesh, caller), packageName.String())

	return oci.AllocatedName{
		Keychain: keychainSession(em),
		ImageID: oci.ImageID{
			Repository: url,
			Tag:        version.String(),
		},
	}, nil
}

func (em ecrManager) AllocateTag(packageName schema.PackageName, buildID provision.BuildID) compute.Computable[oci.AllocatedName] {
	keychain := keychainSession(em)

	var repo compute.Computable[string] = &makeRepository{
		sesh:           em.sesh,
		callerIdentity: keychain.resolveAccount(),
		packageName:    packageName.String(),
	}

	return compute.Map(tasks.Action("ecr.allocate-tag").Category("aws"),
		compute.Inputs().
			Stringer("packageName", packageName).Stringer("buildID", buildID).
			Computable("repo", repo),
		compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (oci.AllocatedName, error) {
			return oci.AllocatedName{
				Keychain: keychain,
				ImageID: oci.ImageID{
					Repository: compute.GetDepValue(deps, repo, "repo"),
					Tag:        buildID.String(),
				},
			}, nil
		},
	)
}

type makeRepository struct {
	sesh           aws.Config
	callerIdentity compute.Computable[*sts.GetCallerIdentityOutput]
	packageName    string

	compute.DoScoped[string] // Can share results within a graph invocation.
}

func (m *makeRepository) Action() *tasks.ActionEvent {
	return tasks.Action("ecr.ensure-repository").Category("aws")
}
func (m *makeRepository) Inputs() *compute.In {
	return compute.Inputs().Computable("caller", m.callerIdentity).Str("packageName", m.packageName)
}
func (m *makeRepository) Compute(ctx context.Context, deps compute.Resolved) (string, error) {
	caller := compute.GetDepValue(deps, m.callerIdentity, "caller")

	req := &ecr.CreateRepositoryInput{
		RepositoryName:     aws.String(m.packageName),
		ImageTagMutability: types.ImageTagMutabilityImmutable,
	}

	if _, err := ecr.NewFromConfig(m.sesh).CreateRepository(ctx, req); err != nil {
		var e *types.RepositoryAlreadyExistsException
		if errors.As(err, &e) {
			// If the repository already exists, that's all good.
		} else {
			return "", fnerrors.InvocationError("failed to create ECR repository for package: %w", err)
		}
	}

	return packageURL(repoURL(m.sesh, caller), m.packageName), nil
}
