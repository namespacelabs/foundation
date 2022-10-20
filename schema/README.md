This directory includes many of the data structures used by Namespace, some of
which are part of the public API, and available to resource providers.

Namespace relies on Protobuf as the data schema language to define our data
types. However, even though we use protobuf serialization in many internal
use-cases, all public APIs rely the JSON form of the same data types, which
follows the formalized proto3 to json conversion schema.

Unfortunately the separation between private and public API is not fully
fleshed out, so you'll see both of them represented here. This will be updated
over the next weeks.
