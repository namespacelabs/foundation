// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"path/filepath"

	"namespacelabs.dev/foundation/schema/storage"
)

func Retain(af *RunningAction) *storage.Command {
	actionId := af.Data.ActionID
	r := &storage.Command{
		ActionLog: []*storage.StoredTask{MakeStoreProto(&af.Data, af.attachments)},
	}

	if af.attachments != nil {
		af.attachments.mu.Lock()
		for _, name := range af.attachments.insertionOrder {
			buf := af.attachments.buffers[name.computed]
			contents := buf.buffer.Snapshot(true)

			r.AttachedLog = append(r.AttachedLog, &storage.Command_Log{
				Id:      filepath.Join(actionId.String(), buf.id),
				Content: contents,
			})
		}
		af.attachments.mu.Unlock()
	}

	return r
}
