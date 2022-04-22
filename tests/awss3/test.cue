import "namespacelabs.dev/foundation/std/fn"

test: fn.#Test & {
	name: "test-aws-s3demoservice"

	binary: {
		from: go_package: "."
	}

	fixture: {
		sut: "namespacelabs.dev/foundation/std/testdata/server/awss3"
	}
}
