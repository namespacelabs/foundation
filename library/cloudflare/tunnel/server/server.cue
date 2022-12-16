server: {
	name: "cloudflare-tunnel"

	imageFrom: binary: "namespacelabs.dev/foundation/library/cloudflare/tunnel/manager"

	args: [
		"--credentials=/cloudflare/tunnel-credentials.json",
		"--configuration=/cloudflare/configuration.yaml",
	]

	services: {
		internal: {
			port: 2000
			kind: "http"
			probe: http: "/ready"
		}
	}

	mounts: "/cloudflare": configurable: {
		contents: {
			"tunnel-credentials.json": fromSecret: ":cfTunnelCredentials"
			"configuration.yaml": fromFile:        "configuration.yaml"
		}
	}
}

secrets: {
	cfTunnelCredentials: {
		description: "Tunnel credentials (as created by cloudflared tunnel create)."
	}
}
