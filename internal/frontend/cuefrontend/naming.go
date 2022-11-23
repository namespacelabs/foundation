// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
)

type cueNaming struct {
	DomainName           map[string][]string `json:"domainName"`
	TLSManagedDomainName map[string][]string `json:"tlsManagedDomainName"`
	WithOrg              string              `json:"withOrg"`
}

func ParseNaming(v *fncue.CueV) (*schema.Naming, error) {
	var data cueNaming
	if err := v.Val.Decode(&data); err != nil {
		return nil, err
	}

	result := &schema.Naming{
		WithOrg: data.WithOrg,
	}

	for k, v := range data.DomainName {
		for _, fqdn := range v {
			result.AdditionalUserSpecified = append(result.AdditionalUserSpecified, &schema.Naming_AdditionalDomainName{
				AllocatedName: k,
				Fqdn:          fqdn,
			})
		}
	}

	for k, v := range data.TLSManagedDomainName {
		for _, fqdn := range v {
			result.AdditionalTlsManaged = append(result.AdditionalTlsManaged, &schema.Naming_AdditionalDomainName{
				AllocatedName: k,
				Fqdn:          fqdn,
			})
		}
	}

	slices.SortFunc(result.AdditionalUserSpecified, sortAdditional)
	slices.SortFunc(result.AdditionalTlsManaged, sortAdditional)

	return result, nil
}
