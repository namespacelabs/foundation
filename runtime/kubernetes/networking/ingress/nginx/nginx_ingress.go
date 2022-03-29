// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nginx

import (
	"context"
	"embed"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeparser"
	"namespacelabs.dev/foundation/schema"
)

var (
	//go:embed ingress.yaml
	lib embed.FS
)

func Ensure(ctx context.Context) ([]kubedef.Apply, error) {
	f, err := lib.Open("ingress.yaml")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	applies, err := kubeparser.FromReader("nginx Ingress", f)
	if err != nil {
		return nil, err
	}

	return applies, nil
}

func IngressAnnotations(hasTLS bool, backendProtocol string, extensions []*anypb.Any) (map[string]string, error) {
	annotations := map[string]string{
		"kubernetes.io/ingress.class":                  "nginx",
		"nginx.ingress.kubernetes.io/backend-protocol": strings.ToUpper(backendProtocol),
	}

	if hasTLS {
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
		annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "true"
	} else {
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "false"
		annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "false"
	}

	var cors *schema.HttpCors

	for _, ext := range extensions {
		switch ext.TypeUrl {
		case "type.googleapis.com/foundation.schema.HttpCors":
			corsConf := &schema.HttpCors{}
			if err := ext.UnmarshalTo(corsConf); err != nil {
				return nil, fnerrors.InternalError("nginx: failed to unpack CORS configuration: %v", err)
			}

			if cors == nil {
				cors = corsConf
			} else if !proto.Equal(cors, corsConf) {
				return nil, fnerrors.InternalError("nginx: incompatible CORS configurations")
			}

		default:
			return nil, fnerrors.InternalError("nginx: don't know how to handle extension %q", ext.TypeUrl)
		}
	}

	if cors != nil {
		// XXX validate allowed origin syntax.

		annotations["nginx.ingress.kubernetes.io/enable-cors"] = "true"
		annotations["nginx.ingress.kubernetes.io/cors-allow-origin"] = strings.Join(cors.AllowedOrigin, ", ")
	}

	return annotations, nil
}

type NameRef struct {
	Namespace, ServiceName string
	ContainerPort          int
}

func IngressLoadBalancerService() *NameRef {
	return &NameRef{
		Namespace:     "ingress-nginx",
		ServiceName:   "ingress-nginx-controller",
		ContainerPort: 80,
	}
}

func ControllerSelector() map[string]string {
	return map[string]string{"app.kubernetes.io/component": "controller"}
}