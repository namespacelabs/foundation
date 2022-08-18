package template

import (
	"namespacelabs.dev/foundation/std/fn"
)

#PostgresServer: fn.#OpaqueServer & {
	isStateful: true

	binary: "namespacelabs.dev/foundation/universe/db/postgres/server/img"

	import: [
		"namespacelabs.dev/foundation/universe/db/postgres/server/creds",
		"namespacelabs.dev/foundation/universe/db/postgres/server/data",
	]

	service: "postgres": {
		containerPort: 5432
		metadata: protocol: "tcp"
	}
}
