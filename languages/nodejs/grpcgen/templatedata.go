// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package grpcgen

import "namespacelabs.dev/foundation/languages/nodejs/imports"

type tmplProtoFile struct {
	Imports  []imports.SingleImport
	Services []tmplProtoService
	Opts     GenOpts
}

type tmplProtoService struct {
	Name    string
	Methods []tmplProtoMethod
}

type tmplProtoMethod struct {
	Name         string
	OriginalName string
	Path         string
	RequestType  tmplImportedType
	ResponseType tmplImportedType
}

type tmplImportedType struct {
	ImportAlias string
	Name        string
}
