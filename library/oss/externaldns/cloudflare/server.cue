server: {
	name:  "externaldns-cloudflare"
	image: "k8s.gcr.io/external-dns/external-dns:v0.7.6"

	args: [
		"--source=service",
		"--provider=cloudflare",
		"--txt-owner-id=$(POD_NAMESPACE)-$(POD_NAME)",
	]

	env: {
		CF_API_TOKEN: fromSecret: ":cfApiToken"

		POD_NAME: experimentalFromDownwardsFieldPath:      "metadata.name"
		POD_NAMESPACE: experimentalFromDownwardsFieldPath: "metadata.namespace"
	}

	// unstable_ is a reference to this API not having been finalized.
	unstable_permissions: {
		clusterRoles: ["namespacelabs.dev/foundation/library/oss/externaldns:permissions"]
	}
}

secrets: {
	cfApiToken: {
		description: "Cloudflare API Token"
	}
}
