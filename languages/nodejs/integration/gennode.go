// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"context"
	"encoding/base64"

	"github.com/iancoleman/strcase"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/languages/shared"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const (
	depsFilename             = "deps.fn.ts"
	packageServiceBaseName   = "Service"
	packageExtensionBaseName = "Extension"
	grpcNpmPackage           = "@grpc/grpc-js"
)

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

	var packageBaseName string
	if nodeData.Kind == schema.Node_SERVICE {
		packageBaseName = packageServiceBaseName
	} else {
		packageBaseName = packageExtensionBaseName
	}

	packageDeps, err := convertDependencyList(ic, packageBaseName, nodeData.Deps)
	if err != nil {
		return nodeTmplOptions{}, err
	}

	var packageDepsName *string
	if packageDeps != nil {
		packageDepsName = &packageBaseName
	}

	var initializer *tmplInitializer
	if nodeData.Initializer != nil {
		initializer = &tmplInitializer{
			PackageDepsName:  packageDepsName,
			InitializeBefore: nodeData.Initializer.InitializeBefore,
			InitializeAfter:  nodeData.Initializer.InitializeAfter,
		}
	}

	providers := []tmplProvider{}
	for _, p := range nodeData.Providers {
		inputType, err := convertType(ic, p.InputType)
		if err != nil {
			return nodeTmplOptions{}, err
		}

		scopeDeps, err := convertDependencyList(ic, p.Name, p.ScopedDeps)
		if err != nil {
			return nodeTmplOptions{}, err
		}

		providers = append(providers, tmplProvider{
			Name:            strcase.ToCamel(p.Name),
			InputType:       inputType,
			OutputType:      convertAvailableIn(ic, p.ProviderType.Nodejs),
			Deps:            scopeDeps,
			PackageDepsName: packageDepsName,
		})
	}

	var service *tmplService
	if nodeData.Kind == schema.Node_SERVICE {
		service = &tmplService{
			GrpcServerImportAlias: ic.add(grpcNpmPackage),
		}
	} else {
		service = nil
	}

	depsImportAliases, err := convertImportedInitializes(ic, nodeData.DepsImportAliases)
	if err != nil {
		return nodeTmplOptions{}, err
	}

	return nodeTmplOptions{
		Imports: ic.imports(),
		Package: tmplPackage{
			Name:              nodeData.PackageName,
			Deps:              packageDeps,
			Initializer:       initializer,
			DepsImportAliases: depsImportAliases,
		},
		Providers: providers,
		Service:   service,
	}, nil
}

func convertDependency(ic *importCollector, dep shared.DependencyData) (tmplDependency, error) {
	npmPackage, err := toNpmPackage(dep.ProviderLocation.PackageName)
	if err != nil {
		return tmplDependency{}, err
	}
	alias := ic.add(nodeDepsNpmImport(npmPackage))

	inputType, err := convertType(ic, dep.ProviderInputType)
	if err != nil {
		return tmplDependency{}, err
	}

	return tmplDependency{
		Name: dep.Name,
		Type: convertAvailableIn(ic, dep.ProviderType.Nodejs),
		Provider: tmplImportedType{
			Name:        strcase.ToCamel(dep.ProviderName),
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
func convertDependencyList(ic *importCollector, name string, deps []shared.DependencyData) (*tmplDeps, error) {
	if deps == nil {
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
