// Release with:
//
// nsdev build-binary internal/networking/ingress/nginx/image \
//     --build_platforms=linux/arm64,linux/amd64 \
//     --base_repository=us-docker.pkg.dev/foundation-344819/prebuilts/

binary: {
	name: "nginx"

	build_plan: [
		{dockerfile: "Dockerfile"},
	]
}
