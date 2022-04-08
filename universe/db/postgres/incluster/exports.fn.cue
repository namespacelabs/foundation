// This file is automatically generated.
package incluster

import (
	"namespacelabs.dev/foundation/std/fn:types"
)

#Exports: {
	Database: {
		name?:       string
		schemaFile?: types.#Resource

		#Definition: {
			packageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster"
			type:        "Database"
			typeDefinition: {
				"typename": "foundation.universe.db.postgres.incluster.Database"
				"source": [
					"provider.proto",
				]
			}
		}
	}
}
