import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

$proto: inputs.#Proto & {
	source: "../proto/list.proto"
}

service: fn.#Service & {
	framework: "GO"

	resources: {
		postgres: {
			class:    "namespacelabs.dev/foundation/library/database/postgres:Database"
			provider: "namespacelabs.dev/foundation/library/oss/postgres"

			intent: {
				name: "list"
				schema: ["schema.sql"]
			}

			resources: {
				"cluster": "namespacelabs.dev/foundation/library/oss/postgres:colocated"
			}
		}
	}

	instantiate: {
		db: postgres.#Exports.Database & {
			resourceRef: "namespacelabs.dev/foundation/internal/testdata/service/list:postgres"
		}
	}

	exportService:        $proto.services.ListService
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
}
