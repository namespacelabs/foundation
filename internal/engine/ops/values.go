package ops

import (
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
)

func Value[M proto.Message](inv *schema.SerializedInvocation, name string) (M, error) {
	var empty M

	for _, x := range inv.Computed {
		if x.Name == name {
			v, err := x.Value.UnmarshalNew()
			if err != nil {
				return empty, err
			}
			return v.(M), nil
		}
	}

	return empty, nil
}
