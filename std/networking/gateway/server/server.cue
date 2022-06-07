import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "sun4qtee50l61888bdj0"
	name: "envoyproxy"

	binary: image: "docker.io/envoyproxy/envoy:v1.22.0"
}
