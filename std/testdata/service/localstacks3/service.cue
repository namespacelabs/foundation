import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/storage/s3"
)

$proto: inputs.#Proto & {
	source: "../proto/file.proto"
}

service: fn.#Service & {
	framework: "GO"

	instantiate: {
		"bucket": s3.#Exports.Bucket & {
			bucketName: "test-foo-bucket"
		}
	}

	exportService: $proto.services.FileService
	ingress:       "INTERNET_FACING"
}
