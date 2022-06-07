import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "sun4qtee50l61888bdj0"
	name: "envoyproxy"

	isStateful: true

	binary: "namespacelabs.dev/foundation/std/networking/envoy/proxy/image"
}
