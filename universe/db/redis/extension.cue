import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/core"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	instantiate: {
		readinessCheck: core.#Exports.ReadinessCheck
	}

	provides: {
		Redis: {
			input: $providerProto.types.RedisArgs

			availableIn: {
				go: {
					package: "github.com/go-redis/redis/v8"
					type:    "*Client"
				}
			}
		}
	}
}

$env: inputs.#Environment

$redisServer: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/universe/db/redis/server"
}

configure: fn.#Configure & {
	stack: {
		append: [$redisServer]
	}

	if $redisServer.$addressMap.redis != _|_ {
		startup: {
			args: {
				redis_endpoint: $redisServer.$addressMap.redis
			}
		}
	}
}
