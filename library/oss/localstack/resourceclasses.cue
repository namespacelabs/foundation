resourceClasses: {
	"Cluster": {
		description: "LocalStack Server"
		intent: {
			type:   "library.oss.localstack.ClusterIntent"
			source: "./types.proto"
		}
		produces: {
			type:   "library.oss.localstack.ClusterInstance"
			source: "./types.proto"
		}
	}
}
