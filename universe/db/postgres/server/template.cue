package template

import (
	"namespacelabs.dev/foundation/std/fn"
)

#PostgresServer: fn.#OpaqueServer & {
	isStateful: true

	binary: "namespacelabs.dev/foundation/universe/db/postgres/server/img"

	service: "postgres": {
		containerPort: 5432
		metadata: protocol: "tcp"
	}
}
