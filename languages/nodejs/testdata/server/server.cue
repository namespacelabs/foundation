import "namespacelabs.dev/foundation/std/fn"

server: fn.#Server & {
	id:        "bkpqlg4jt9fgo01bqam0"
	name:      "test-nodejs-server"
	framework: "NODEJS"

	import: [
		"namespacelabs.dev/foundation-nodejs-testdata/services/simple",
		"namespacelabs.dev/foundation-nodejs-testdata/services/numberformatter",
	]
}
