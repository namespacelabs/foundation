import "namespacelabs.dev/foundation/std/fn"

test: fn.#Test & {
	name: "test-localstack-s3"

	binary: {
		from: go_package: "."
	}

	fixture: {
		sut: "namespacelabs.dev/foundation/internal/testdata/server/localstacks3"
	}
}
