import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "sun4qtee50l61888bdj0"
	name: "envoyproxy"

	isStateful: true

	binary: image: "docker.io/envoyproxy/envoy:v1.22.0"

	import: [
		"namespacelabs.dev/foundation/std/networking/envoy/proxy/datastorage",
	]
}

configure: fn.#Configure & {
	startup: {
		args: ["-c", "/config/envoy.json"]
	}
}
