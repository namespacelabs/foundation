// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"bytes"
	"io"
	"sync"

	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/syncbuffer"
	"namespacelabs.dev/go-ids"
)

type EventAttachments struct {
	actionID ActionID
	sink     ActionSink

	mu sync.Mutex // Protects below.
	ResultData
	// For the time being we just keep everything in memory for simplicity.
	buffers        map[string]attachedBuffer
	insertionOrder []OutputName
}

type readerWriter interface {
	Writer() io.Writer
	Reader() io.ReadCloser
	Snapshot() []byte
}

type attachedBuffer struct {
	buffer      readerWriter
	writer      io.Writer
	id          string
	name        string
	contentType string
}

func (ev *EventAttachments) IsRecording() bool { return ev != nil }
func (ev *EventAttachments) IsStoring() bool   { return ActionStorer != nil }

func (ev *EventAttachments) init() {
	if ev.buffers == nil {
		ev.buffers = map[string]attachedBuffer{}
	}
}

func (ev *EventAttachments) seal() {
	ev.mu.Lock()
	defer ev.mu.Unlock()

	for name, b := range ev.buffers {
		if cb, ok := b.buffer.(*syncbuffer.ByteBuffer); ok {
			ev.buffers[name] = attachedBuffer{
				id:          ids.NewRandomBase62ID(8),
				buffer:      cb.Seal(),
				name:        b.name,
				contentType: b.contentType,
			}
		}
	}
}

func (ev *EventAttachments) attach(name OutputName, body []byte) {
	ev.mu.Lock()
	defer ev.mu.Unlock()

	ev.init()

	if _, ok := ev.buffers[name.computed]; !ok {
		ev.insertionOrder = append(ev.insertionOrder, name)
	}

	ev.buffers[name.computed] = attachedBuffer{
		id:          ids.NewRandomBase62ID(8),
		buffer:      syncbuffer.Seal(body),
		name:        name.name,
		contentType: name.contentType,
	}
}

func (ev *EventAttachments) Attach(name OutputName, body []byte) {
	if ev != nil {
		ev.attach(name, body)
		ev.sink.AttachmentsUpdated(ev.actionID, nil)
	}
}

func (ev *EventAttachments) AttachSerializable(name, modifier string, body interface{}) error {
	if ev == nil {
		return fnerrors.InternalError("no running action while attaching serializable %q", name)
	}

	if !ev.IsRecording() {
		return nil
	}

	bytes, err := common.SerializeToBytes(body)
	if err != nil {
		return fnerrors.BadInputError("failed to serialize payload to bytes: %w", err)
	}

	contentType := "application/json"
	if modifier != "" {
		contentType += "+" + modifier
	}

	ev.Attach(Output(name, contentType), bytes)
	return nil
}

func (ev *EventAttachments) addResult(key string, msg interface{}) ResultData {
	ev.mu.Lock()
	defer ev.mu.Unlock()

	for _, item := range ev.Items {
		if item.Name == key {
			item.Msg = msg
			return ev.ResultData
		}
	}

	// Not found, add a new result.
	ev.Items = append(ev.Items, &ActionArgument{key, msg})
	return ev.ResultData
}

func (ev *EventAttachments) AddResult(key string, msg interface{}) *EventAttachments {
	if ev != nil {
		data := ev.addResult(key, msg)
		// XXX this is racy as we don't guarantee the AttachmentsUpdated order if the caller
		// is using multiple go-routines to update outputs.
		ev.sink.AttachmentsUpdated(ev.actionID, &data)
	}

	return ev
}

func (ev *EventAttachments) SetProgress(p ActionProgress) *EventAttachments {
	if ev != nil {
		ev.mu.Lock()
		ev.Progress = p
		copy := ev.ResultData
		ev.mu.Unlock()

		ev.sink.AttachmentsUpdated(ev.actionID, &copy)
	}

	return ev
}

func (ev *EventAttachments) ReaderByOutputName(outputName OutputName) io.ReadCloser {
	return ev.ReaderByName(outputName.name)
}

func (ev *EventAttachments) ReaderByName(name string) io.ReadCloser {
	if ev != nil {
		ev.mu.Lock()
		defer ev.mu.Unlock()

		for _, b := range ev.buffers {
			if b.name == name {
				return b.buffer.Reader()
			}
		}
	}

	return io.NopCloser(bytes.NewReader(nil))
}

func (ev *EventAttachments) ensureOutput(name OutputName, outputType common.CatOutputType, addIfMissing bool) (io.Writer, bool) {
	if ev == nil {
		return syncbuffer.Discard, false
	}

	ev.mu.Lock()
	defer ev.mu.Unlock()

	if !addIfMissing && ev.buffers == nil {
		return syncbuffer.Discard, false
	}

	ev.init()

	added := false
	if _, ok := ev.buffers[name.computed]; !ok {
		if !addIfMissing {
			return syncbuffer.Discard, false
		}

		buf := syncbuffer.NewByteBuffer()
		out := buf.Writer()

		if sinkOutput := ev.sink.Output(name.name, name.contentType, outputType); sinkOutput != nil {
			out = io.MultiWriter(out, sinkOutput)
		}

		ev.buffers[name.computed] = attachedBuffer{
			id:          ids.NewRandomBase62ID(8),
			buffer:      buf,
			writer:      out,
			name:        name.name,
			contentType: name.contentType,
		}

		ev.insertionOrder = append(ev.insertionOrder, name)
		added = true
	}

	return ev.buffers[name.computed].writer, added
}

func (ev *EventAttachments) Output(name OutputName, cat common.CatOutputType) io.Writer {
	w, added := ev.ensureOutput(name, cat, true)
	if added {
		ev.sink.AttachmentsUpdated(ev.actionID, nil)
	}
	return w
}

func (ev *EventAttachments) ActionID() ActionID {
	if ev == nil {
		return ""
	}
	return ev.actionID
}
