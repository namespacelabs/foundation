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

const depsFilename = "deps.fn.ts"
const singletonName = "Singleton"

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

	return generateSource(ctx, fs, loc.Rel(depsFilename), tmpl, "Node", tmplOptions)
}

func convertNodeDataToTmplOptions(nodeData shared.NodeData) (nodeTmplOptions, error) {
	ic := newImportCollector()

	providers := []tmplProvider{}
	for _, p := range nodeData.Providers {
		inputType, err := convertType(ic, p.InputType)
		if err != nil {
			return nodeTmplOptions{}, err
		}

		scopeDeps, err := convertDependencies(ic, p.Name, p.ScopedDeps)
		if err != nil {
			return nodeTmplOptions{}, err
		}

		providers = append(providers, tmplProvider{
			Name:       capitalCaser.String(p.Name),
			InputType:  inputType,
			OutputType: convertAvailableIn(ic, p.ProviderType.Nodejs),
			ScopedDeps: scopeDeps,
		})
	}

	var service *tmplDeps
	if nodeData.Service != nil {
		deps, err := convertDependencies(ic, singletonName, nodeData.Service.Deps)
		if err != nil {
			return nodeTmplOptions{}, err
		}

		service = deps
	}

	return nodeTmplOptions{
		Imports:   ic.imports(),
		Service:   service,
		Providers: providers,
	}, nil
}

func convertDependency(ic *importCollector, dep shared.DependencyData) (tmplDependency, error) {
	npmPackage, err := toNpmPackage(dep.ProviderLocation.PackageName)
	if err != nil {
		return tmplDependency{}, err
	}
	alias := ic.add(nodeDepsNpmImport(npmPackage))

	inputType, err := convertType(ic, dep.Provider.InputType)
	if err != nil {
		return tmplDependency{}, err
	}

	return tmplDependency{
		Name: dep.Name,
		Type: convertAvailableIn(ic, dep.Provider.ProviderType.Nodejs),
		Provider: tmplImportedType{
			Name:        capitalCaser.String(dep.Provider.Name),
			ImportAlias: alias,
		},
		ProviderInputType: inputType,
		ProviderInput: tmplSerializedProto{
			Base64Content: base64.StdEncoding.EncodeToString(dep.ProviderInput.Content),
			Comments:      dep.ProviderInput.Comments,
		},
	}, nil
}

// Returns nil if the input list is empty.
func convertDependencies(ic *importCollector, name string, deps []shared.DependencyData) (*tmplDeps, error) {
	if len(deps) == 0 {
		return nil, nil
	}

	convertedDeps := []tmplDependency{}
	for _, d := range deps {
		dep, err := convertDependency(ic, d)
		if err != nil {
			return nil, err
		}

		convertedDeps = append(convertedDeps, dep)
	}

	return &tmplDeps{
		Name: name,
		Deps: convertedDeps,
	}, nil
}
