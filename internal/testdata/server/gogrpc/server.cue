import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

server: fn.#Server & {
	id:        "7hzne001dff2rpdxav703bwqwc"
	name:      "gogrpcserver"
	framework: "GO"

	import: [
		"namespacelabs.dev/foundation/internal/testdata/service/post",
		"namespacelabs.dev/foundation/std/grpc/logging",
		"namespacelabs.dev/foundation/std/monitoring/prometheus",
		"namespacelabs.dev/foundation/std/monitoring/tracing/jaeger",
		"namespacelabs.dev/foundation/universe/go/panicparse",
		"namespacelabs.dev/foundation/universe/aws/irsa",
	]
}

configure: fn.#Configure & {
	naming: {
		if $env.purpose != "TESTING" {
			domainName: "grpc-gateway-7hzne001dff2rpdxav703bwqwc": ["test.namespacelabs.net"]
		}
	}
}
