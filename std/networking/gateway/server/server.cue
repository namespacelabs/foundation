import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "sun4qtee50l61888bdj0"
	name: "gateway"

	binary: image: "docker.io/envoyproxy/envoy:v1.22.0"
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/std/networking/gateway/server/configure"
}
