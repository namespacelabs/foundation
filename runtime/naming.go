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

var NamingNoTLS = false // Set to true in CI.

var errLogin = fnerrors.UsageError("Please run `fn login` to login.",
	"Foundation automatically manages nscloud.dev-based sub-domains and issues SSL certificates on your behalf. To use these features, you'll need to login to Foundation using your Github account.")

// GuessAllocatedName does not RPC out to figure out what names we would use.
// If we do end up being able to deploy, the names are correct, but we just
// don't know ahead of time we will be able to use them.
func GuessAllocatedName(env *schema.Environment, srv *schema.Server, naming *schema.Naming, name string) (*schema.Domain, error) {
	if env.Purpose != schema.Environment_PRODUCTION {
		return &schema.Domain{
			Fqdn:    fmt.Sprintf("%s.%s.%s", name, env.Name, LocalBaseDomain),
			Managed: schema.Domain_LOCAL_MANAGED,
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

	if env.Purpose == schema.Environment_PRODUCTION {
		if orgOverride := naming.GetWithOrg(); orgOverride != "" {
			org = orgOverride
		}
	}

	return &schema.Domain{
		// XXX include stack?
		Fqdn:    fmt.Sprintf("%s.%s.%s.%s", name, env.Name, org, CloudBaseDomain),
		Managed: schema.Domain_CLOUD_MANAGED,
	}, nil
}

func allocateName(ctx context.Context, srv *schema.Server, naming *schema.Naming, startopts fnapi.AllocateOpts) (*schema.Domain_Certificate, error) {
	var cacheKey string

	if startopts.Subdomain != "" {
		if startopts.Org == "" {
			return nil, fnerrors.InternalError("%s: org must be specified", startopts.Subdomain)
		}
		cacheKey = startopts.Subdomain
	} else if startopts.FQDN != "" {
		cacheKey = startopts.FQDN + ".specific"
	} else {
		return nil, fnerrors.BadInputError("either FQDN or Subdomain must be set")
	}

	previous, _ := checkStored(ctx, srv, startopts.Org, cacheKey)
	if previous != nil && isResourceValid(previous) {
		// We ignore errors.
		return certFromResource(previous), nil
	}

	startopts.NoTLS = NamingNoTLS
	startopts.Stored = previous

	nr, err := fnapi.AllocateName(ctx, srv, startopts)
	if err != nil {
		return nil, err
	}

	if err := storeCert(ctx, srv, startopts.Org, cacheKey, nr); err != nil {
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
	return nr.FQDN != "" && nr.Certificate.PrivateKey != nil && nr.Certificate.CertificateBundle != nil
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
