// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"google.golang.org/protobuf/encoding/prototext"
)

type Storer struct {
	bundler *Bundler
	bundle  *Bundle
}

func NewStorer(ctx context.Context, bundler *Bundler, bundle *Bundle) (*Storer, error) {
	return &Storer{bundler, bundle}, nil
}

func (st *Storer) Store(af *RunningAction) {
	if err := st.store(af); err != nil {
		// XXX log warnings
		return
	}
}

func (st *Storer) store(af *RunningAction) error {
	actionId := af.Data.ActionID

	pbytes, err := prototext.MarshalOptions{Multiline: true}.Marshal(makeDebugProto(&af.Data, af.attachments))
	if err != nil {
		return err
	}

	if err := st.bundle.WriteFile(context.Background(), filepath.Join(actionId, "action.textpb"), pbytes, 0600); err != nil {
		return err
	}

	if af.attachments != nil {
		af.attachments.mu.Lock()
		for k, name := range af.attachments.insertionOrder {
			id := fmt.Sprintf("%d", k)
			buf := af.attachments.buffers[name.computed]

			out, err := ioutil.ReadAll(buf.buffer.Reader())
			if err != nil {
				return err
			}
			if err := st.bundle.WriteFile(context.Background(), filepath.Join(actionId, id+filepath.Ext(buf.name)), out, 0600); err != nil {
				return err
			}
		}
		af.attachments.mu.Unlock()
	}
	return nil
}

func (st *Storer) RecoverFromPanic(ctx context.Context, err error) {
	// We flush the bundle even if we had an error serializing the
	// error.
	_ = st.bundle.WriteErrorWithStacktrace(ctx, err)
	st.bundler.Flush(ctx, st.bundle)
}
