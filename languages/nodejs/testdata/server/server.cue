import "namespacelabs.dev/foundation/std/fn"

server: fn.#Server & {
	id:        "bkpqlg4jt9fgo01bqam0"
	name:      "test-nodejs-server"
	framework: "NODEJS"

	import: [
		"namespacelabs.dev/foundation/languages/nodejs/http",
		"namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple",
		"namespacelabs.dev/foundation/languages/nodejs/testdata/services/simplehttp",
		"namespacelabs.dev/foundation/languages/nodejs/testdata/services/numberformatter",
		"namespacelabs.dev/foundation/languages/nodejs/testdata/services/postuser",
	]
}
