resourceClasses: {
	"Database": {
		intent: {
			type:   "foundation.integrations.testdata.resources.classes.DatabaseIntent"
			source: "./proto1.proto"
		}
		produces: {
			type:   "foundation.integrations.testdata.resources.classes.protos.DatabaseInstance"
			source: "./protos/proto2.proto"
		}
	}
}
