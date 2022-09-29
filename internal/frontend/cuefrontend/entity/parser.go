// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package entity

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

// Calls one of the registered entityt parsers, repending on the value of the "urlCueKey" field.
func NewDispatchingEntityParser(urlCueKey string, parsers []EntityParser) *DispatchingEntityParser {
	registeredParsers := map[string]EntityParser{}
	for _, p := range parsers {
		registeredParsers[p.Url()] = p
	}
	return &DispatchingEntityParser{
		urlCueKey:         urlCueKey,
		registeredParsers: registeredParsers,
	}
}

type DispatchingEntityParser struct {
	urlCueKey string
	// Key: url
	registeredParsers map[string]EntityParser
}

type ParsedEntity struct {
	Url  string
	Data proto.Message
}

func (p *DispatchingEntityParser) ParseEntity(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (ParsedEntity, error) {
	// First checking for the full cueUrl
	if cueUrl := v.LookupPath(p.urlCueKey); cueUrl.Exists() {
		url, err := cueUrl.Val.String()
		if err != nil {
			return ParsedEntity{}, err
		}

		if i, ok := p.registeredParsers[url]; ok {
			return parse(ctx, pl, loc, i, v)
		} else {
			return ParsedEntity{}, fnerrors.UserError(loc, "unknown url: %s", url)
		}
	}

	// If the kind is not specified, trying the short form, e.g.:
	//   integration: golang {
	//	   pkg: "."
	//   }
	for _, p := range p.registeredParsers {
		if shortV := v.LookupPath(p.Shortcut()); shortV.Exists() {
			return parse(ctx, pl, loc, p, shortV)
		}
		// Shortest form:
		//  integration: "golang"
		if str, err := v.Val.String(); err == nil && str == p.Shortcut() {
			return parse(ctx, pl, loc, p, nil)
		}
	}

	return ParsedEntity{}, fnerrors.UserError(loc, "%q content is not recognized, neither a full form nor a shorcut", v.Val.Path())
}

func parse(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, p EntityParser, v *fncue.CueV) (ParsedEntity, error) {
	data, err := p.Parse(ctx, pl, loc, v)
	if err != nil {
		return ParsedEntity{}, err
	}
	return ParsedEntity{
		Url:  p.Url(),
		Data: data,
	}, nil
}
