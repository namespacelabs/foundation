// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/certificates"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/dirs"
)

const (
	LocalBaseDomain = "nslocal.host"
	CloudBaseDomain = "nscloud.dev"
)

var (
	NamingNoTLS             = false // Set to true in CI.
	ReuseStoredCertificates = true

	WorkInProgressUseShortAlias = false
)

var errLogin = fnerrors.UsageError("Please run `ns login` to login.",
	"Namespace automatically manages nscloud.dev-based sub-domains and issues SSL certificates on your behalf. To use these features, you'll need to login to Namespace using your Github account.")

func ComputeNaming(ctx context.Context, ws string, env planning.Context, cluster Planner, source *schema.Naming) (*schema.ComputedNaming, error) {
	result, err := computeNaming(ctx, ws, env, cluster, source)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(console.Debug(ctx), "computed naming: %+v\n", result)

	return result, nil
}

func computeNaming(ctx context.Context, workspace string, env planning.Context, cluster Planner, source *schema.Naming) (*schema.ComputedNaming, error) {
	naming, err := computeInnerNaming(ctx, env, cluster, source)
	if err != nil {
		return nil, err
	}

	naming.MainModuleName = workspace
	naming.UseShortAlias = naming.UseShortAlias || WorkInProgressUseShortAlias

	return naming, nil
}

func computeInnerNaming(ctx context.Context, rootenv planning.Context, cluster Planner, source *schema.Naming) (*schema.ComputedNaming, error) {
	base, err := cluster.ComputeBaseNaming(source)
	if err != nil {
		return nil, err
	}

	if base != nil {
		return base, nil
	}

	env := rootenv.Environment()

	if env.Purpose != schema.Environment_PRODUCTION {
		return &schema.ComputedNaming{
			Source:     source,
			BaseDomain: LocalBaseDomain,
			Managed:    schema.Domain_LOCAL_MANAGED,
		}, nil
	}

	userAuth, err := fnapi.LoadUser()
	if err != nil {
		if errors.Is(err, fnapi.ErrRelogin) {
			return nil, errLogin
		}

		return nil, err
	}

	org := userAuth.Org
	if org == "" {
		org = userAuth.Username
	}

	if env.Purpose == schema.Environment_PRODUCTION {
		if orgOverride := source.GetWithOrg(); orgOverride != "" {
			org = orgOverride
		}
	}

	return &schema.ComputedNaming{
		Source:     source,
		BaseDomain: fmt.Sprintf("%s.%s", org, CloudBaseDomain),
		Managed:    schema.Domain_CLOUD_MANAGED,
	}, nil
}

func allocateName(ctx context.Context, srv Deployable, opts fnapi.AllocateOpts) (*schema.Certificate, error) {
	var cacheKey string

	if opts.Subdomain != "" {
		if opts.Org == "" {
			return nil, fnerrors.InternalError("%s: org must be specified", opts.Subdomain)
		}
		cacheKey = opts.Subdomain + ".wildcard"
	} else if opts.FQDN != "" {
		cacheKey = opts.FQDN
	} else {
		return nil, fnerrors.BadInputError("either FQDN or Subdomain must be set")
	}

	previous, _ := checkStored(ctx, srv, opts.Org, cacheKey)
	if ReuseStoredCertificates {
		if previous != nil && isResourceValid(previous) {
			// We ignore errors.
			return certFromResource(previous), nil
		}
	}

	opts.NoTLS = NamingNoTLS
	opts.Stored = previous
	opts.Scope = schema.PackageName(srv.GetPackageName())

	nr, err := fnapi.AllocateName(ctx, opts)
	if err != nil {
		return nil, err
	}

	if err := storeCert(ctx, srv, opts.Org, cacheKey, nr); err != nil {
		fmt.Fprintf(console.Warnings(ctx), "failed to persistent certificate for cacheKey=%s: %v\n", cacheKey, err)
	}

	return certFromResource(nr), nil
}

func certFromResource(res *fnapi.NameResource) *schema.Certificate {
	if res.Certificate.PrivateKey != nil && res.Certificate.CertificateBundle != nil {
		return &schema.Certificate{
			PrivateKey:        res.Certificate.PrivateKey,
			CertificateBundle: res.Certificate.CertificateBundle,
		}
	}

	return nil
}

func isResourceValid(nr *fnapi.NameResource) bool {
	if nr.Certificate.PrivateKey != nil && nr.Certificate.CertificateBundle != nil {
		valid, _, _ := certificates.CertIsValid(nr.Certificate.CertificateBundle)
		return valid
	}

	return false
}

func checkStored(ctx context.Context, srv Deployable, org, cacheKey string) (*fnapi.NameResource, error) {
	// XXX security check permissions
	certDir, err := makeCertDir(org, srv)
	if err != nil {
		return nil, err
	}

	// XXX security check escape
	contents, err := ioutil.ReadFile(filepath.Join(certDir, cacheKey+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var nr fnapi.NameResource
	if err := json.Unmarshal(contents, &nr); err != nil {
		return nil, err
	}

	return &nr, nil
}

func storeCert(ctx context.Context, srv Deployable, org, cacheKey string, res *fnapi.NameResource) error {
	certDir, err := makeCertDir(org, srv)
	if err != nil {
		return err
	}

	resBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(certDir, os.ModeDir|0700); err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(certDir, cacheKey+".json"), resBytes, 0600)
}

func makeCertDir(org string, srv Deployable) (string, error) {
	if org == "" {
		return "", fnerrors.New("no org specified")
	}

	certDir, err := dirs.CertCache()
	if err != nil {
		return "", err
	}

	return filepath.Join(certDir, org, srv.GetId()), nil
}
