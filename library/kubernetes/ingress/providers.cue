providers: {
	"namespacelabs.dev/foundation/library/runtime:Ingress": {
		prepareWith: "namespacelabs.dev/foundation/library/kubernetes/ingress/prepare"

		intent: {
			type:   "library.kubernetes.ingress.IngressIntent"
			source: "./types.proto"
		}
	}
}
