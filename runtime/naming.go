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
	"namespacelabs.dev/foundation/workspace/dirs"
)

const (
	LocalBaseDomain = "nslocal.host"
	CloudBaseDomain = "nscloud.dev"
)

var (
	NamingNoTLS             = false // Set to true in CI.
	ReuseStoredCertificates = true

	WorkInProgressComputedDomainSuffixBasedNames = false
)

var errLogin = fnerrors.UsageError("Please run `ns login` to login.",
	"Namespace automatically manages nscloud.dev-based sub-domains and issues SSL certificates on your behalf. To use these features, you'll need to login to Namespace using your Github account.")

func ComputeNaming(ctx context.Context, env *schema.Environment, source *schema.Naming) (*schema.ComputedNaming, error) {
	result, err := computeNaming(env, source)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(console.Debug(ctx), "computed naming: %+v\n", result)

	return result, nil
}

func computeNaming(env *schema.Environment, source *schema.Naming) (*schema.ComputedNaming, error) {
	if WorkInProgressComputedDomainSuffixBasedNames {
		return &schema.ComputedNaming{
			// XXX get information from cluster.
			Source:                  source,
			BaseDomain:              "a.nscluster.cloud",
			Managed:                 schema.Domain_CLOUD_MANAGED,
			TlsFrontend:             true,
			TlsInclusterTermination: false,
			DomainFragmentSuffix:    "880g-674g3ttig51ajfl6l343b5jcuo",
		}, nil
	}

	if env.Purpose != schema.Environment_PRODUCTION {
		return &schema.ComputedNaming{
			Source:                  source,
			BaseDomain:              LocalBaseDomain,
			Managed:                 schema.Domain_LOCAL_MANAGED,
			TlsFrontend:             false,
			TlsInclusterTermination: false,
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
		Source:                  source,
		BaseDomain:              fmt.Sprintf("%s.%s", org, CloudBaseDomain),
		Managed:                 schema.Domain_CLOUD_MANAGED,
		TlsFrontend:             true,
		TlsInclusterTermination: true,
	}, nil
}

func allocateName(ctx context.Context, srv *schema.Server, opts fnapi.AllocateOpts) (*schema.Domain_Certificate, error) {
	var cacheKey string

	if opts.Subdomain != "" {
		if opts.Org == "" {
			return nil, fnerrors.InternalError("%s: org must be specified", opts.Subdomain)
		}
		cacheKey = opts.Subdomain
	} else if opts.FQDN != "" {
		cacheKey = opts.FQDN + ".specific"
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
	opts.Scope = schema.PackageName(srv.PackageName)

	nr, err := fnapi.AllocateName(ctx, opts)
	if err != nil {
		return nil, err
	}

	if err := storeCert(ctx, srv, opts.Org, cacheKey, nr); err != nil {
		fmt.Fprintf(console.Warnings(ctx), "failed to persistent certificate for cacheKey=%s: %v\n", cacheKey, err)
	}

	return certFromResource(nr), nil
}

func certFromResource(res *fnapi.NameResource) *schema.Domain_Certificate {
	if res.Certificate.PrivateKey != nil && res.Certificate.CertificateBundle != nil {
		return &schema.Domain_Certificate{
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

func checkStored(ctx context.Context, srv *schema.Server, org, cacheKey string) (*fnapi.NameResource, error) {
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

func storeCert(ctx context.Context, srv *schema.Server, org, cacheKey string, res *fnapi.NameResource) error {
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

func makeCertDir(org string, srv *schema.Server) (string, error) {
	if org == "" {
		return "", fnerrors.New("no org specified")
	}

	certDir, err := dirs.CertCache()
	if err != nil {
		return "", err
	}

	return filepath.Join(certDir, org, srv.Id), nil
}
