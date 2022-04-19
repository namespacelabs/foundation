// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/encoding/prototext"
)

type Storer struct{ baseDir string }

func NewStorer(ctx context.Context) (*Storer, error) {
	t, err := os.MkdirTemp("", "fn-actions")
	if err != nil {
		return nil, err
	}

	return &Storer{t}, nil
}

func (st *Storer) Store(af *RunningAction) {
	if err := st.store(af); err != nil {
		// XXX log warnings
		return
	}
}

func (st *Storer) store(af *RunningAction) error {
	target := filepath.Join(st.baseDir, af.Data.ActionID)
	if err := os.Mkdir(target, 0700); err != nil {
		return err
	}

	pbytes, err := prototext.MarshalOptions{Multiline: true}.Marshal(makeDebugProto(&af.Data, af.attachments))
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(target, "action.textpb"), pbytes, 0600); err != nil {
		return err
	}

	for k, name := range af.attachments.insertionOrder {
		id := fmt.Sprintf("%d", k)
		buf := af.attachments.buffers[name.computed]

		out, err := ioutil.ReadAll(buf.buffer.Reader())
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(filepath.Join(target, id+filepath.Ext(buf.name)), out, 0600); err != nil {
			return err
		}
	}

	return nil
}

func (st *Storer) Flush(w io.Writer) {
	fmt.Fprintf(w, "Stored actions at: %s\n", st.baseDir)
}
