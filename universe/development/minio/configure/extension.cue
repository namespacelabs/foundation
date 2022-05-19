import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension

configure: fn.#Configure & {
	startup: {
		args: [
			"server",
			"/tmp",
			"--address=:9000",
			"--console-address=:9001",
		]
	}
}
