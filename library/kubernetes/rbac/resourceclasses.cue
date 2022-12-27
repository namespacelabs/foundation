resourceClasses: {
	"ClusterRole": {
		produces: {
			type:   "library.kubernetes.rbac.ClusterRoleInstance"
			source: "./rbac.proto"
		}
		defaultProvider: "namespacelabs.dev/foundation/library/kubernetes/rbac/providers"
	}
}
