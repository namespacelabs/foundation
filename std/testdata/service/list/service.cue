import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
)

$proto: inputs.#Proto & {
	source: "../proto/list.proto"
}

service: fn.#Service & {
	framework: "GO"

	instantiate: {
		db: incluster.#Exports.Database & {
			name:       "list"
			schemaFile: inputs.#FromFile & {
				path: "schema.sql"
			}
		}
	}

	exportService:        $proto.services.ListService
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
}
