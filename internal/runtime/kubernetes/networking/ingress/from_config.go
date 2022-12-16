package ingress

import (
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/std/cfg"
)

func FromConfig(cfg cfg.Configuration) kubedef.IngressClass {
	return nginx.Ingress()
}
