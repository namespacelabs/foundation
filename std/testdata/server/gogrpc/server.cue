import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

server: fn.#Server & {
	id:        "7hzne001dff2rpdxav703bwqwc"
	name:      "gogrpcserver"
	framework: "GO_GRPC"

	import: [
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/post",
		"namespacelabs.dev/foundation/std/grpc/logging",
	]
}

$env:      inputs.#Environment
configure: fn.#Configure & {
	naming: {
		if $env.purpose == "PRODUCTION" {
			domainName: "test.namespacelabs.net"
		}
	}
}
