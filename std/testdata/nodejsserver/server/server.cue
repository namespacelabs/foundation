import "namespacelabs.dev/foundation/std/fn"

server: fn.#Server & {
	id:        "0acbwfdsbn17mvk9ry3q2x37kh"
	name:      "nodejsgrpcserver"
	framework: "NODEJS_GRPC"

	import: [
		// TODO remove after this is implicit.
		"namespacelabs.dev/foundation/std/nodejs/grpc"
	]
}
