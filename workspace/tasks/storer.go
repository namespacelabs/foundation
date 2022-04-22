// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/internal/fnfs"
)

type Storer struct{ bundle *Bundle }

func NewStorer(ctx context.Context, bundle *Bundle) (*Storer, error) {
	return &Storer{bundle}, nil
}

func (st *Storer) Store(af *RunningAction) {
	if err := st.store(af); err != nil {
		// XXX log warnings
		return
	}
}

func (st *Storer) store(af *RunningAction) error {
	if mkdirfs, ok := st.bundle.fsys.(fnfs.MkdirFS); ok {
		err := mkdirfs.MkdirAll(af.Data.ActionID, 0700)
		if err != nil {
			return err
		}
	}

	pbytes, err := prototext.MarshalOptions{Multiline: true}.Marshal(makeDebugProto(&af.Data, af.attachments))
	if err != nil {
		return err
	}

	if err := fnfs.WriteFile(context.Background(), st.bundle.fsys, "action.textpb", pbytes, 0600); err != nil {
		return err
	}
	
	for k, name := range af.attachments.insertionOrder {
		id := fmt.Sprintf("%d", k)
		buf := af.attachments.buffers[name.computed]

		out, err := ioutil.ReadAll(buf.buffer.Reader())
		if err != nil {
			return err
		}

		if err := fnfs.WriteFile(context.Background(), st.bundle.fsys, id+filepath.Ext(buf.name), out, 0600); err != nil {
			return err
		}
	}

	return nil
}

func (st *Storer) Flush(w io.Writer) {
	fmt.Fprintf(w, "Stored actions at: %s\n", st.bundle.root)
}
