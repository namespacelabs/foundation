resources: {
	permissions: {
		class: "namespacelabs.dev/foundation/library/kubernetes/rbac:ClusterRole"
		intent: {
			name: "ExternalDNS"
			rules: [
				{apiGroups: [""], resources: ["services", "endpoints", "pods"], verbs: ["get", "watch", "list"]},
				{apiGroups: ["extensions", "networking.k8s.io"], resources: ["ingresses"], verbs: ["get", "watch", "list"]},
				{apiGroups: [""], resources: ["nodes"], verbs: ["watch", "list"]},
			]
		}
	}
}
