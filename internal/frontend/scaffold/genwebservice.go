// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package scaffold

import (
	"context"
	"fmt"
	"os"
	"strings"

	"namespacelabs.dev/foundation/internal/fnfs"
)

type GenWebServiceOpts struct {
}

func CreateWebServiceScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenWebServiceOpts) error {
	parts := strings.Split(loc.RelPath, string(os.PathSeparator))

	if len(parts) < 1 {
		return fmt.Errorf("unable to determine package name")
	}

	tmplOpts := webServiceTmplOptions{}

	for _, tmplFile := range webTemplates {
		if err := generateWebSource(ctx, fsfs, loc.Rel(tmplFile.filename), tmplFile.tmpl, tmplOpts); err != nil {
			return err
		}
	}
	return nil
}
