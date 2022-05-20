import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "5ifbjqpna8jiqukphvqg"
	name:      "minio"
	framework: "GO_GRPC"

	import: [
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/minio",
	]
}
