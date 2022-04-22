import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/development/localstack/s3"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
	framework: "GO_GRPC"

	instantiate: {
		"bucket": s3.#Exports.Bucket & {
			region:     "us-east-2"
			bucketName: "test-foo-bucket"
		}
	}

	exportService: $proto.services.S3DemoService
	ingress:       "INTERNET_FACING"
}
