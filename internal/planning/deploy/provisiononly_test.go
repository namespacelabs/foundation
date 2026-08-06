// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"testing"

	"gotest.tools/assert"
	internalresources "namespacelabs.dev/foundation/internal/resources"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	stdresources "namespacelabs.dev/foundation/std/resources"
)

func TestKeepProvisionOnlyServers(t *testing.T) {
	app := schema.MakePackageSingleRef("example.com/app")
	pg := schema.MakePackageSingleRef("example.com/postgres")
	other := schema.MakePackageSingleRef("example.com/other")

	specs := []runtime.DeployableSpec{
		{PackageRef: app},
		{PackageRef: pg},
		{PackageRef: other},
		{PackageRef: nil},
	}

	kept := keepProvisionOnlyServers(specs, []schema.PackageName{pg.AsPackageName()})

	assert.Equal(t, len(kept), 1)
	assert.Equal(t, kept[0].PackageRef.AsPackageName(), pg.AsPackageName())
}

func TestProvisionOnlyOutputSinkEmpty(t *testing.T) {
	sink, err := provisionOnlyOutputSink(nil)
	assert.NilError(t, err)
	assert.Assert(t, sink == nil)
}

func TestProvisionOnlyOutputSink(t *testing.T) {
	ids := []string{"res-a", "res-b"}

	sink, err := provisionOnlyOutputSink(ids)
	assert.NilError(t, err)
	assert.Assert(t, sink != nil)

	// Every produced output must be consumed to keep the plan's accounting balanced.
	assert.DeepEqual(t, sink.RequiredOutput, ids)

	assert.DeepEqual(t, sink.Order.SchedAfterCategory, []string{
		stdresources.ResourceInstanceCategory("res-a"),
		stdresources.ResourceInstanceCategory("res-b"),
	})

	msg := &internalresources.OpConsumeResourceOutputs{}
	assert.NilError(t, sink.Impl.UnmarshalTo(msg))
	assert.DeepEqual(t, msg.ResourceInstanceId, ids)
}
