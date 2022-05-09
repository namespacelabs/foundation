import (
"namespacelabs.dev/foundation/std/fn"
 "namespacelabs.dev/foundation/std/testdata/server/modeling")

test: fn.#Test & {
	name: "test-scoped-instantiation"

	binary: {
		from: go_package: "."
	}

	fixture: {
		sut: "namespacelabs.dev/foundation/std/testdata/server/modeling"
	}
}
