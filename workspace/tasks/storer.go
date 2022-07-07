// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"google.golang.org/protobuf/encoding/prototext"
)

type Storer struct {
	bundler   *Bundler
	bundle    *Bundle
	flushLogs func()
}

func NewStorer(ctx context.Context, bundler *Bundler, bundle *Bundle, options ...func(*Storer)) *Storer {
	storer := &Storer{bundler: bundler, bundle: bundle}
	for _, option := range options {
		option(storer)
	}
	return storer
}

func StorerWithFlushLogs(flushLogs func()) func(*Storer) {
	return func(storer *Storer) {
		storer.flushLogs = flushLogs
	}
}

func (st *Storer) Store(af *RunningAction) {
	if err := st.store(af); err != nil {
		// XXX log warnings
		return
	}
}

func (st *Storer) store(af *RunningAction) error {
	actionId := af.Data.ActionID

	pbytes, err := prototext.MarshalOptions{Multiline: true}.Marshal(makeStoreProto(&af.Data, af.attachments))
	if err != nil {
		return err
	}

	if err := st.bundle.WriteFile(context.Background(), filepath.Join(actionId.String(), "action.textpb"), pbytes, 0600); err != nil {
		return err
	}

	if af.attachments != nil {
		af.attachments.mu.Lock()
		for _, name := range af.attachments.insertionOrder {
			buf := af.attachments.buffers[name.computed]

			out, err := ioutil.ReadAll(buf.buffer.Reader())
			if err != nil {
				return err
			}
			if err := st.bundle.WriteFile(context.Background(), filepath.Join(actionId.String(), buf.id), out, 0600); err != nil {
				return err
			}
		}
		af.attachments.mu.Unlock()
	}
	return nil
}

func (st *Storer) WriteRuntimeStack(ctx context.Context, stack []byte) error {
	return st.bundle.WriteFile(ctx, "runtime_stack.txt", stack, 0600)
}

func (st *Storer) Flush(ctx context.Context) error {
	if st.flushLogs != nil {
		st.flushLogs()
	}
	return st.bundler.Flush(ctx, st.bundle)
}
