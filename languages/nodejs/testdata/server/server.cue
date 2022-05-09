import (
"namespacelabs.dev/foundation/std/fn"
		"namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple",
		"namespacelabs.dev/foundation/languages/nodejs/testdata/services/numberformatter",
        )

server: fn.#Server & {
	id:        "bkpqlg4jt9fgo01bqam0"
	name:      "test-nodejs-server"
	framework: "NODEJS"

	import: [
		"namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple",
		"namespacelabs.dev/foundation/languages/nodejs/testdata/services/numberformatter",
	]
}
