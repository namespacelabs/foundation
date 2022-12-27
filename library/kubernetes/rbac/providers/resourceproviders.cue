providers: {
	"namespacelabs.dev/foundation/library/kubernetes/rbac:ClusterRole": {
		prepareWith: "namespacelabs.dev/foundation/library/kubernetes/rbac/prepare/clusterrole"

		intent: {
			type:   "library.kubernetes.rbac.ClusterRoleIntent"
			source: "../rbac.proto"
		}
	}
}
