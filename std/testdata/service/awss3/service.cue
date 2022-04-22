import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/aws/s3"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
	framework: "GO_GRPC"

	instantiate: {
		"bucket": s3.#Exports.Bucket & {
			region:     "us-east-2"
			bucketName: "ns-foundation-foo-bucket-test3"
		}
	}

	exportService:        $proto.services.S3DemoService
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
}
