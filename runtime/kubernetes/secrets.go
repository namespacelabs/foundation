// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r k8sRuntime) FetchSecret(ctx context.Context, name string) (map[string][]byte, error) {
	secret, err := r.cli.CoreV1().Secrets(r.moduleNamespace).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return secret.Data, nil
}
