// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"fmt"
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime/constants"
)

func TestParsePersistentVolumeMounts(t *testing.T) {
	for _, tt := range []struct {
		name       string
		field      string
		accessMode string
		template   bool
		wantErr    bool
	}{
		{name: "omitted mode"},
		{name: "empty mode", field: `access_mode: ""`},
		{name: "ReadWriteOnce", field: `access_mode: "ReadWriteOnce"`, accessMode: "ReadWriteOnce"},
		{name: "ReadWriteMany", field: `access_mode: "ReadWriteMany"`, accessMode: "ReadWriteMany"},
		{name: "ReadOnlyMany", field: `access_mode: "ReadOnlyMany"`, accessMode: "ReadOnlyMany"},
		{name: "ReadWriteOncePod", field: `access_mode: "ReadWriteOncePod"`, accessMode: "ReadWriteOncePod"},
		{name: "template", field: `access_mode: "ReadWriteMany"`, accessMode: "ReadWriteMany", template: true},
		{name: "typo", field: `access_mode: "ReadWriteManny"`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			loc := pkggraph.NewLocationForTesting(nil, "example.com/storage", "storage")
			v := &fncue.CueV{Val: cuecontext.New().CompileString(fmt.Sprintf(`mounts: {
				"/data": {persistent: {
					id: "shared"
					size: "1TB"
					storage_class: "truenas-ssd-nfs"
					template: %t
					%s
				}}
			}`, tt.template, tt.field))}
			if !assert.NoError(t, v.Val.Err()) {
				return
			}
			mounts, volumes, err := ParseMounts(context.Background(), nil, loc, v.LookupPath("mounts"))
			if tt.wantErr {
				assert.ErrorContains(t, err, "ReadWriteManny")
				assert.ErrorContains(t, err, "access_mode")
				assert.ErrorContains(t, err, "storage:")
				return
			}
			if !assert.NoError(t, err) || !assert.Len(t, mounts, 1) || !assert.Len(t, volumes, 1) {
				return
			}
			mount, volume := mounts[0], volumes[0]
			assert.Equal(t, "/data", mount.Path)
			assert.Equal(t, schema.MakePackageRef(loc.PackageName, volume.Name), mount.VolumeRef)
			assert.Equal(t, "example.com/storage", volume.Owner)
			assert.Equal(t, constants.VolumeKindPersistent, volume.Kind)
			assert.True(t, volume.Inline)
			persistent := &schema.PersistentVolume{}
			if !assert.NoError(t, volume.Definition.UnmarshalTo(persistent)) {
				return
			}
			assert.Equal(t, "shared", persistent.Id)
			assert.Equal(t, uint64(1_000_000_000_000), persistent.SizeBytes)
			assert.Equal(t, "truenas-ssd-nfs", persistent.StorageClass)
			assert.Equal(t, tt.template, persistent.Template)
			assert.Equal(t, tt.accessMode, persistent.AccessMode)
		})
	}
}
