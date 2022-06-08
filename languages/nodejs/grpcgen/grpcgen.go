// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package grpcgen

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/languages/nodejs/imports"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

type GenOpts struct {
	GenClients bool
	GenServers bool
}

func Generate(fdp *descriptor.FileDescriptorProto, deps *protos.FileDescriptorSetAndDeps, opts GenOpts) ([]byte, error) {
	if !opts.GenClients && !opts.GenServers {
		return nil, fnerrors.InternalError("Grpc Coodegen: both GenClients and GenServers options can't be false")
	}

	if len(fdp.Service) == 0 {
		return nil, nil
	}

	file := tmplProtoFile{
		Services: []tmplProtoService{},
		Opts:     opts,
	}

	ic := imports.NewImportCollector()

	for _, svc := range fdp.Service {
		svcTmpl := tmplProtoService{
			Name:    svc.GetName(),
			Methods: []tmplProtoMethod{},
		}

		for _, mtd := range svc.Method {
			requestType, err := convertMessageType(ic, fdp, deps, mtd.GetInputType())
			if err != nil {
				return nil, err
			}

			responseType, err := convertMessageType(ic, fdp, deps, mtd.GetOutputType())
			if err != nil {
				return nil, err
			}

			methodName := strings.ToLower(string(mtd.GetName()[0])) + mtd.GetName()[1:]

			mtdTmpl := tmplProtoMethod{
				Name:         methodName,
				OriginalName: mtd.GetName(),
				RequestType:  requestType,
				ResponseType: responseType,
				Path:         fmt.Sprintf("/%s.%s/%s", fdp.GetPackage(), svc.GetName(), mtd.GetName()),
			}
			svcTmpl.Methods = append(svcTmpl.Methods, mtdTmpl)
		}

		file.Services = append(file.Services, svcTmpl)
	}

	file.Imports = ic.Imports()

	var b bytes.Buffer
	if err := tmpl.ExecuteTemplate(&b, "File", file); err != nil {
		return nil, fnerrors.InternalError("failed to apply template: %w", err)
	}
	return b.Bytes(), nil
}

func convertMessageType(ic *imports.ImportCollector, fdp *descriptor.FileDescriptorProto, deps *protos.FileDescriptorSetAndDeps, fullMessageType string) (tmplImportedType, error) {
	parts := strings.Split(fullMessageType, ".")
	messageType := parts[len(parts)-1]

	msgFile, _ := protos.LookupDescriptorProto(deps, fullMessageType)
	relPath, err := filepath.Rel(filepath.Dir(fdp.GetName()), filepath.Dir(msgFile.GetName()))
	if err != nil {
		return tmplImportedType{}, err
	}

	protoTsFn := strings.TrimSuffix(filepath.Base(msgFile.GetName()), ".proto") + "_pb"
	importPath := filepath.Join(relPath, protoTsFn)
	if !strings.HasPrefix(importPath, ".") {
		importPath = "./" + importPath
	}
	return tmplImportedType{
		ImportAlias: ic.Add(importPath),
		Name:        messageType,
	}, nil
}
