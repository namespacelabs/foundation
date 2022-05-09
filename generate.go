package p

// topoSort - why there is 'after'

// protos - opgen node opprotogen opgenserver opgennode

func x() {
	// type Location struct {
	// 	Module      *Module
	// 	PackageName schema.PackageName

	// 	relPath string
	// }
	// type Location struct {
	// 	Module      *Module
	// 	PackageName schema.PackageName

	// 	relPath string
	// }
	for _, loc := range locs {
		//_ := &fnfs.Location {ModuleName: "namespacelabs.dev/foundation",
		// *{root: "/home/kamil/namespacelabs/foundation", readWrite: true, announceWritesTo: io.Writer nil}
		// RelPath: "devworkflow/web"}
		sealed, err := workspace.Seal(ctx, pl, loc.AsPackageName(), nil)
		// type Sealed struct {
		// 	Location      Location
		// 	Proto         *schema.Stack_Entry
		// 	FileDeps      []string
		// 	Deps          []*Package
		// 	ParsedPackage *Package
		// }
		//
		// message Stack {
		//     repeated Entry            entry             = 1;
		//     repeated Endpoint         endpoint          = 2;
		//     repeated InternalEndpoint internal_endpoint = 3;

		//     message Entry {
		//         Server        server        = 1;
		//         Naming        server_naming = 3;
		//         repeated Node node          = 2;
		//     }
		// }

	}
}

// server  (stack_entry)
// package_name: "namespacelabs.dev/foundation/std/testdata/server/localstacks3"
// id: "rn5q3mcug1dnkbtue3cg"
// name: "supporttestserver"
// import: "namespacelabs.dev/foundation/std/go/core"
// import: "namespacelabs.dev/foundation/std/go/grpc/interceptors"
// import: "namespacelabs.dev/foundation/std/monitoring/grafana"
// import: "namespacelabs.dev/foundation/std/monitoring/prometheus"
// import: "namespacelabs.dev/foundation/std/go/grpc/metrics"
// import: "namespacelabs.dev/foundation/std/go/grpc"
// import: "namespacelabs.dev/foundation/std/go/grpc/gateway"
// import: "namespacelabs.dev/foundation/universe/development/localstack/s3"
// import: "namespacelabs.dev/foundation/std/testdata/service/localstacks3"
// namespace_module: "namespacelabs.dev/foundation"
// ext: <
//   type_url: "type.googleapis.com/foundation.languages.golang.FrameworkExt"
//   value: "\n\0041.18\022\034namespacelabs.dev/foundation\032\001."
// >
// allocation: <
//   instance: <
//     instance_owner: "namespacelabs.dev/foundation/std/go/grpc/metrics"
//     package_name: "namespacelabs.dev/foundation/std/go/grpc/interceptors"
//     alloc_name: "0"
//     instantiated: <
//       package_name: "namespacelabs.dev/foundation/std/go/grpc/interceptors"
//       type: "InterceptorRegistration"
//       name: "interceptors"
//       constructor: <
//         type_url: "type.foundation.namespacelabs.dev/namespacelabs.dev/foundation/std/go/grpc/interceptors/foundation.std.go.grpc.interceptors.InterceptorRegistration"
//       >
//     >
//   >
// >
// allocation: <
//   instance: <
//     instance_owner: "namespacelabs.dev/foundation/std/testdata/service/localstacks3"
//     package_name: "namespacelabs.dev/foundation/universe/development/localstack/s3"
//     alloc_name: "1"
//     instantiated: <
//       package_name: "namespacelabs.dev/foundation/universe/development/localstack/s3"
//       type: "Bucket"
//       name: "bucket"
//       constructor: <
//         type_url: "type.foundation.namespacelabs.dev/namespacelabs.dev/foundation/universe/development/localstack/s3/foundation.universe.aws.s3.BucketConfig"
//         value: "\n\tus-east-2\022\017test-foo-bucket"
//       >
//     >
//     downstream_allocation: <
//       instance: <
//         instance_owner: "namespacelabs.dev/foundation/universe/development/localstack/s3"
//         package_name: "namespacelabs.dev/foundation/std/go/core"
//         alloc_name: "1.2"
//         instantiated: <
//           package_name: "namespacelabs.dev/foundation/std/go/core"
//           type: "ReadinessCheck"
//           name: "readinessCheck"
//           constructor: <
//             type_url: "type.foundation.namespacelabs.dev/namespacelabs.dev/foundation/std/go/core/foundation.std.go.core.ReadinessCheckArgs"
//           >
//         >
//       >
//     >
//   >
// >
// allocation: <
//   instance: <
//     instance_owner: "namespacelabs.dev/foundation/universe/development/localstack/s3"
//     package_name: "namespacelabs.dev/foundation/std/go/core"
//     alloc_name: "3"
//     instantiated: <
//       package_name: "namespacelabs.dev/foundation/std/go/core"
//       type: "ReadinessCheck"
//       name: "readinessCheck"
//       constructor: <
//         type_url: "type.foundation.namespacelabs.dev/namespacelabs.dev/foundation/std/go/core/foundation.std.go.core.ReadinessCheckArgs"
//       >
//     >
//   >
// >
// framework: GO_GRPC
// user_imports: "namespacelabs.dev/foundation/std/go/grpc/gateway"
// user_imports: "namespacelabs.dev/foundation/std/testdata/service/localstacks3"

type _checkProvideDeadlines func(context.Context, *Deadline) (*DeadlineRegistration, error)

var _ _checkProvideDeadlines = ProvideDeadlines

var (
	Package__vbko45 = &core.Package{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
	}

	Initializers__vbko45 = []*core.Initializer{
		{
			Package: Package__vbko45,
			Do: func(ctx context.Context, di core.Dependencies) error {
				return di.Instantiate(ctx, Provider__vbko45, func(ctx context.Context, v interface{}) error {
					return Prepare(ctx, v.())
				})
			},
		},
	}
)
