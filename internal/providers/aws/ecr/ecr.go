// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ecr

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
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/internal/providers/aws"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/tasks"
)

type ecrManager struct {
	sesh *awsprovider.Session
}

var _ registry.Manager = ecrManager{}

func Register() {
	registry.Register("aws/ecr", func(ctx context.Context, ck planning.Configuration) (m registry.Manager, finalErr error) {
		sesh, err := awsprovider.MustConfiguredSession(ctx, ck)
		if err != nil {
			return nil, err
		}

		return ecrManager{sesh: sesh}, nil
	})

	oci.RegisterDomainKeychain("amazonaws.com", DefaultKeychain, oci.Keychain_UseAlways)
}

func packageURL(repo, packageName string) string {
	return fmt.Sprintf("%s/%s", repo, packageName)
}

func repoURL(sesh aws.Config, caller *sts.GetCallerIdentityOutput) string {
	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", *caller.Account, sesh.Region)
}

func (em ecrManager) IsInsecure() bool { return false }

func (em ecrManager) Tag(ctx context.Context, packageName schema.PackageName) (oci.AllocatedName, error) {
	res, err := compute.Get(ctx, keychainSession(em).resolveAccount())
	if err != nil {
		return oci.AllocatedName{}, err
	}

	caller := res.Value
	url := packageURL(repoURL(em.sesh.Config(), caller), packageName.String())

	return oci.AllocatedName{
		Keychain: keychainSession(em),
		ImageID: oci.ImageID{
			Repository: url,
		},
	}, nil
}

func (em ecrManager) AllocateName(repository string) compute.Computable[oci.AllocatedName] {
	keychain := keychainSession(em)

	var repo compute.Computable[string] = &makeRepository{
		sesh:           em.sesh,
		callerIdentity: keychain.resolveAccount(),
		repository:     repository,
	}

	return compute.Map(tasks.Action("ecr.allocate-tag").Category("aws"),
		compute.Inputs().Str("repository", repository).Computable("repo", repo),
		compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (oci.AllocatedName, error) {
			imgid := oci.ImageID{
				Repository: compute.MustGetDepValue(deps, repo, "repo"),
			}

			tasks.Attachments(ctx).AddResult("repository", imgid.Repository)

			return oci.AllocatedName{
				Keychain: keychain,
				ImageID:  imgid,
			}, nil
		},
	)
}

func (em ecrManager) AuthRepository(img oci.ImageID) (oci.AllocatedName, error) {
	keychain := keychainSession(em)

	return oci.AllocatedName{
		Keychain: keychain,
		ImageID:  img,
	}, nil
}

type makeRepository struct {
	sesh           *awsprovider.Session
	callerIdentity compute.Computable[*sts.GetCallerIdentityOutput]
	repository     string

	compute.DoScoped[string] // Can share results within a graph invocation.
}

func (m *makeRepository) Action() *tasks.ActionEvent {
	return tasks.Action("ecr.ensure-repository").Category("aws").Arg("repository", m.repository)
}

func (m *makeRepository) Inputs() *compute.In {
	return compute.Inputs().Computable("caller", m.callerIdentity).Str("packageName", m.repository).Str("cacheKey", m.sesh.CacheKey())
}

func (m *makeRepository) Compute(ctx context.Context, deps compute.Resolved) (string, error) {
	caller := compute.MustGetDepValue(deps, m.callerIdentity, "caller")

	req := &ecr.CreateRepositoryInput{
		RepositoryName:     aws.String(m.repository),
		ImageTagMutability: types.ImageTagMutabilityImmutable,
	}

	if _, err := ecr.NewFromConfig(m.sesh.Config()).CreateRepository(ctx, req); err != nil {
		var e *types.RepositoryAlreadyExistsException
		if errors.As(err, &e) {
			// If the repository already exists, that's all good.
		} else {
			return "", fnerrors.InvocationError("%s: failed to create ECR repository: %w", m.repository, err)
		}
	}

	return packageURL(repoURL(m.sesh.Config(), caller), m.repository), nil
}
