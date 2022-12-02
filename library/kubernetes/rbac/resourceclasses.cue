resourceClasses: {
	"ClusterRole": {
		intent: {
			type:   "library.kubernetes.rbac.ClusterRoleIntent"
			source: "./rbac.proto"
		}
		produces: {
			type:   "library.kubernetes.rbac.ClusterRoleInstance"
			source: "./rbac.proto"
		}
		defaultProvider: "namespacelabs.dev/foundation/library/kubernetes/rbac/providers"
	}
}
