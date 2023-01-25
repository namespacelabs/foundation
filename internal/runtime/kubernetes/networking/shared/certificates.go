// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package shared

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/tools/maketlscert"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
)

func AllocateDomainCertificate(ctx context.Context, env *schema.Environment, entry *schema.Stack_Entry, template *schema.Domain) (*schema.Certificate, error) {
	if env.Purpose == schema.Environment_PRODUCTION {
		return allocateName(ctx, entry.Server, fnapi.AllocateOpts{
			FQDN: template.Fqdn,
			// XXX remove org -- it should be parsed from fqdn.
			Org: entry.ServerNaming.GetWithOrg(),
		})
	} else {
		bundle, err := maketlscert.CreateSelfSignedCertificateChain(ctx, env, &types.TLSCertificateSpec{
			Description: entry.Server.PackageName,
			DnsName:     []string{template.Fqdn},
		})
		if err != nil {
			return nil, err
		}
		return bundle.Server, nil
	}
}

func allocateName(ctx context.Context, srv runtime.Deployable, opts fnapi.AllocateOpts) (*schema.Certificate, error) {
	opts.NoTLS = runtime.NamingNoTLS
	opts.Scope = srv.GetPackageRef().AsPackageName()

	nr, err := fnapi.AllocateName(ctx, opts)
	if err != nil {
		return nil, err
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
