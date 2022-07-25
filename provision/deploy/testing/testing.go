// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"bytes"
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gotest.tools/assert"
)

// golang protojson doesn't guarantee stability:
// https://developers.google.com/protocol-buffers/docs/reference/go/faq#hyrums_law
func StableProtoToJson(t *testing.T, p proto.Message) string {
	btes, err := (protojson.MarshalOptions{
		UseProtoNames: true,
	}).Marshal(p)
	assert.NilError(t, err)

	var dst bytes.Buffer
	err = json.Indent(&dst, btes, "", "  ")
	assert.NilError(t, err)
	return dst.String()
}
