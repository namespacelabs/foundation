binary: {
	name: "cloudflare-tunnel-manager"

	build_plan: {
		layer_build_plan: [
			{prebuilt: "cloudflare/cloudflared@sha256:79bd6f5fbcaf17a0955e9e82b05323419105ae841afa905c5bf1d455b9aebcce"},
			{go_build: {
				rel_path:    "."
				binary_name: "cloudflare-tunnel-manager"
				binary_only: true
			}},
		]
	}
}
