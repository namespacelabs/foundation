// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"encoding/base64"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/languages/shared"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const DepsFilename = "deps.fn.ts"

var capitalCaser = cases.Title(language.AmericanEnglish)

func generateNode(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, nodes []*schema.Node, fs fnfs.ReadWriteFS) error {
	nodeData, err := shared.PrepareNodeData(ctx, loader, loc, n, schema.Framework_NODEJS)
	if err != nil {
		return err
	}

	tmplOptions, err := convertNodeDataToTmplOptions(nodeData)
	if err != nil {
		return err
	}

	return generateSource(ctx, fs, loc.Rel(DepsFilename), serviceTmpl, tmplOptions)
}

func convertNodeDataToTmplOptions(nodeData shared.NodeData) (nodeTmplOptions, error) {
	ic := newImportCollector()

	providers := []tmplProvider{}
	for _, p := range nodeData.Providers {
		inputType, err := convertType(ic, p.InputType)
		if err != nil {
			return nodeTmplOptions{}, err
		}

		providers = append(providers, tmplProvider{
			Name:       capitalCaser.String(p.Name),
			InputType:  inputType,
			OutputType: convertAvailableIn(ic, p.ProviderType.Nodejs),
		})
	}

	var service *tmplService
	if nodeData.Service != nil {
		deps := []tmplDependency{}
		for _, d := range nodeData.Service.Deps {
			npmPackage, err := toNpmPackage(d.ProviderLocation.PackageName)
			if err != nil {
				return nodeTmplOptions{}, err
			}
			alias := ic.add(nodeDepsNpmImport(npmPackage))

			inputType, err := convertType(ic, d.Provider.InputType)
			if err != nil {
				return nodeTmplOptions{}, err
			}

			deps = append(deps, tmplDependency{
				Name: d.Name,
				Type: convertAvailableIn(ic, d.Provider.ProviderType.Nodejs),
				Provider: tmplImportedType{
					Name:        d.Provider.Name,
					ImportAlias: alias,
				},
				ProviderInputType: inputType,
				ProviderInput: tmplSerializedProto{
					Base64Content: base64.StdEncoding.EncodeToString(d.ProviderInput.Content),
					Comments:      d.ProviderInput.Comments,
				},
			})
		}

		service = &tmplService{
			Deps: deps,
		}
	}

	return nodeTmplOptions{
		Imports:   ic.imports(),
		Service:   service,
		Providers: providers,
	}, nil
}
