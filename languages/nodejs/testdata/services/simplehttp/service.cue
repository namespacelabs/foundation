import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

service: fn.#Service & {
	framework: "NODEJS"

	// curl -X POST http://127.0.0.1:40001/simple/123
	//
	// {"output":"Hello world! User ID: 123"}
	exportHttp: [{path: "/simple"}]
}
