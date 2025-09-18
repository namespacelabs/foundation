import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

$proto: inputs.#Proto & {
	source: "../proto/list.proto"
}

service: fn.#Service & {
	framework:     "GO"
	exportService: $proto.services.ListService

	resources: {
		testRoachDb: {
			class:    "namespacelabs.dev/foundation/library/database/cockroach:Database"
			provider: "namespacelabs.dev/foundation/library/oss/cockroach"

			intent: {
				name: "list"
				schema: ["schema.sql"]
				regions: ["local"]
				survivalGoal: "zone"
			}

			resources: {
				"cluster": "namespacelabs.dev/foundation/library/oss/cockroach:colocated"
			}
		}
	}

	instantiate: {
		db: postgres.#Exports.Database & {
			resourceRef: "namespacelabs.dev/foundation/internal/testdata/service/roachlist:testRoachDb"
		}
	}

	exportService: $proto.services.ListService
	ingress:       "INTERNET_FACING"
}
