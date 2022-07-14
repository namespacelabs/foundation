// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

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
var ReuseStoredCertificates = true

var errLogin = fnerrors.UsageError("Please run `ns login` to login.",
	"Namespace automatically manages nscloud.dev-based sub-domains and issues SSL certificates on your behalf. To use these features, you'll need to login to Namespace using your Github account.")

func ComputeNaming(env *schema.Environment, source *schema.Naming) (*schema.ComputedNaming, error) {
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

	nr, err := fnapi.AllocateName(ctx, srv, opts)
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
	if nr.FQDN != "" && nr.Certificate.PrivateKey != nil && nr.Certificate.CertificateBundle != nil {
		valid, _, _ := certIsValid(nr.Certificate.CertificateBundle)
		return valid
	}

	return false
}

func certIsValid(bundle []byte) (bool, time.Time, error) {
	now := time.Now()

	// The rest is ignored, as we only care about the first pem block.
	block, _ := pem.Decode(bundle)
	if block == nil || block.Type != "CERTIFICATE" {
		return false, now, fnerrors.BadInputError("expected CERTIFICATE block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, now, fnerrors.BadInputError("invalid certificate")
	}

	return now.Add(30 * 24 * time.Hour).Before(cert.NotAfter), cert.NotAfter, nil
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
