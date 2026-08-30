import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension

configure: fn.#Configure & {
	startup: {
		env: {
			SERVICES: "s3"
		}
	}
}
