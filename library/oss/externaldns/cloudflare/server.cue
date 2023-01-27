server: {
	name:  "externaldns-cloudflare"
	image: "k8s.gcr.io/external-dns/external-dns:v0.7.6"

	args: [
		"--source=service",
		"--provider=cloudflare",
		"--txt-owner-id=$(POD_NAMESPACE)-$(POD_NAME)",
	]

	// This works around the fact that we only have the API token available in
	// production, so that tests pass. In fact, what we should do instead is
	// disable the startup test for this server, as anyway it doesn't have any
	// readiness probes so we can't verify that it does indeed start.
	if $env.purpose == "PRODUCTION" || $env.name == "dev-cluster" {
		env: CF_API_TOKEN: fromSecret: ":cfApiToken"
	}

	env: {
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
