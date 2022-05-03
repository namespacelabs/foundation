// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/syncbuffer"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks/protocol"
	"namespacelabs.dev/go-ids"
)

var ActionStorer *Storer = nil

type ActionSink interface {
	Waiting(*RunningAction)
	Started(*RunningAction)
	Done(*RunningAction)
	Instant(*EventData)
	AttachmentsUpdated(string, *ResultData)
}

type ActionState string

const (
	ActionCreated = "fn.action.created"
	ActionWaiting = "fn.action.waiting"
	ActionRunning = "fn.action.running"
	ActionDone    = "fn.action.done"
	ActionInstant = "fn.action.instant"
)

func (a ActionState) IsRunning() bool { return a == ActionWaiting || a == ActionRunning }
func (a ActionState) IsDone() bool    { return a == ActionDone || a == ActionInstant }

type OnDoneFunc func(*protocol.Task)

type WellKnown string

const (
	WkAction   = "action"
	WkCategory = "category"
	WkModule   = "module"
	WkRuntime  = "tool-runtime"
)

type EventData struct {
	ActionID      string
	ParentID      string
	AnchorID      string // This action represents "waiting" on the action represented by `anchorID`.
	State         ActionState
	Name          string
	HumanReadable string // If not set, name is used.
	Category      string
	Created       time.Time
	Started       time.Time
	Completed     time.Time
	Arguments     []ActionArgument
	Scope         schema.PackageList
	Level         int
	Indefinite    bool
	Err           error
}

type ActionEvent struct {
	data      EventData
	progress  ActionProgress
	onDone    OnDoneFunc
	wellKnown map[WellKnown]string
}

type attachedBuffer struct {
	buffer      readerWriter
	name        string
	contentType string
}

type readerWriter interface {
	Writer() io.Writer
	Reader() io.ReadCloser
}

type ResultData struct {
	Items    []*ActionArgument
	Progress ActionProgress
}

type EventAttachments struct {
	actionID string
	sink     ActionSink

	mu sync.Mutex // Protects below.
	ResultData
	// For the time being we just keep everything in memory for simplicity.
	buffers        map[string]attachedBuffer
	insertionOrder []OutputName
}

type RunningAction struct {
	Data     EventData
	Progress ActionProgress

	sink        ActionSink
	span        trace.Span
	attachments *EventAttachments
	onDone      OnDoneFunc
}

type ActionArgument struct {
	Name string
	Msg  interface{}
}

func allocEvent() *ActionEvent {
	return &ActionEvent{}
}

func Action(name string) *ActionEvent {
	ev := allocEvent()
	ev.data.Name = name
	ev.data.State = ActionCreated
	return ev
}

func (ev *ActionEvent) HumanReadablef(label string, args ...interface{}) *ActionEvent {
	if len(args) == 0 {
		ev.data.HumanReadable = label
	} else {
		ev.data.HumanReadable = fmt.Sprintf(label, args...)
	}
	return ev
}

func (ev *ActionEvent) Serializable(name string, msg common.SerializableArgument) *ActionEvent {
	ev.data.Arguments = append(ev.data.Arguments, ActionArgument{Name: name, Msg: msg})
	return ev
}

func (ev *ActionEvent) OnDone(f OnDoneFunc) *ActionEvent {
	ev.onDone = f
	return ev
}

func (ev *ActionEvent) ID(id string) *ActionEvent {
	ev.data.ActionID = id
	return ev
}

func (ev *ActionEvent) Anchor(id string) *ActionEvent {
	ev.data.AnchorID = id
	return ev
}

func (ev *ActionEvent) StartTimestamp(ts time.Time) *ActionEvent {
	ev.data.Started = ts
	return ev
}

func (ev *ActionEvent) Category(category string) *ActionEvent {
	ev.data.Category = category
	return ev
}

func (ev *ActionEvent) Parent(tid string) *ActionEvent {
	ev.data.ParentID = tid
	return ev
}

func (ev *ActionEvent) Scope(pkgs ...schema.PackageName) *ActionEvent {
	ev.data.Scope.AddMultiple(pkgs...)
	return ev
}

// Register a well known property, used internally only (e.g. for throttling purposes).
func (ev *ActionEvent) WellKnown(key WellKnown, value string) *ActionEvent {
	if ev.wellKnown == nil {
		ev.wellKnown = map[WellKnown]string{}
	}
	ev.wellKnown[key] = value
	return ev
}

func NewActionID() string { return ids.NewRandomBase62ID(16) }

func (ev *ActionEvent) initMissing() {
	if ev.data.ActionID == "" {
		ev.data.ActionID = NewActionID()
	}
	ev.data.Created = time.Now()
}

// Sets the level for this action (by default it's zero). The lower the level, the higher the importance.
func (ev *ActionEvent) LogLevel(level int) *ActionEvent {
	ev.data.Level = level
	return ev
}

func (ev *ActionEvent) Arg(name string, msg interface{}) *ActionEvent {
	ev.data.Arguments = append(ev.data.Arguments, ActionArgument{Name: name, Msg: msg})
	return ev
}

type ActionProgress interface {
	FormatProgress() string
}

func (ev *ActionEvent) Progress(p ActionProgress) *ActionEvent {
	ev.progress = p
	return ev
}

func (ev *ActionEvent) Indefinite() *ActionEvent {
	ev.data.Indefinite = true
	return ev
}

func (ev *ActionEvent) Clone(makeName func(string) string) *ActionEvent {
	copy := &ActionEvent{
		data: ev.data,
	}

	name := copy.data.Name
	if copy.data.Category != "" {
		name = copy.data.Category + "::" + name
		copy.data.Category = ""
	}

	copy.data.Name = makeName(name)
	return copy
}

func (ev *ActionEvent) toAction(ctx context.Context, state ActionState) *RunningAction {
	sink := SinkFrom(ctx)
	if sink == nil {
		return nil
	}

	var parent *RunningAction
	action := ctx.Value(_actionKey)
	if action != nil {
		parent = action.(*RunningAction)
		ev.data.ParentID = parent.Data.ActionID
	}

	ev.initMissing()

	ev.data.State = state

	span := startSpan(ctx, ev.data)

	return &RunningAction{
		sink:        sink,
		Data:        ev.data,
		span:        span,
		Progress:    ev.progress,
		attachments: &EventAttachments{actionID: ev.data.ActionID, sink: sink},
		onDone:      ev.onDone,
	}
}

func (ev *ActionEvent) Start(ctx context.Context) *RunningAction {
	ra := ev.toAction(ctx, ActionRunning)
	ra.markStarted(ctx)
	return ra
}

type RunOptions struct {
	Wait func(context.Context, map[WellKnown]string) (bool, error)
	Run  func(context.Context) error
}

func (ev *ActionEvent) CheckCacheRun(ctx context.Context, options RunOptions) error {
	ra := ev.toAction(ctx, ActionWaiting)
	ra.sink.Waiting(ra)

	var wasCached bool
	err := ra.Call(ctx, func(ctx context.Context) error {
		if _, ok := ev.wellKnown[WkAction]; !ok {
			if ev.wellKnown == nil {
				ev.wellKnown = map[WellKnown]string{}
			}
			ev.wellKnown[WkAction] = ev.data.Name
		}

		cached, err := options.Wait(ctx, ev.wellKnown)
		if err != nil {
			return err
		}
		wasCached = cached
		return nil
	})
	if err != nil {
		return ra.Done(err)
	}

	if ra.span != nil && ra.span.IsRecording() {
		ra.span.AddEvent("starting", trace.WithAttributes(attribute.Bool("cached", wasCached)))
	}

	if wasCached {
		ra.Data.Arguments = append(ra.Data.Arguments, ActionArgument{Name: "cached", Msg: true})
		return ra.Done(nil)
	}

	// Our data model implies that the caller always owns data; and sinks should perform copies.
	ra.Data.Started = time.Now()
	ra.markStarted(ctx)

	return ra.Done(ra.Call(ctx, options.Run))
}

func (ev *ActionEvent) Run(ctx context.Context, f func(context.Context) error) error {
	defer func() {
		if r := recover(); r != nil {
			// Capture the stack on panic.
			_ = ActionStorer.WriteRuntimeStack(ctx, debug.Stack())

			// Ensure that we always have an audit trail.
			_ = ActionStorer.Flush(ctx)

			// Bubble up the panic.
			panic(r)
		}
	}()
	v := ev.Start(ctx)
	return v.Done(v.Call(ctx, f))
}

func Return[V any](ctx context.Context, ev *ActionEvent, f func(context.Context) (V, error)) (V, error) {
	v := ev.Start(ctx)
	var ret V
	callErr := v.Call(ctx, func(ctx context.Context) error {
		var err error
		ret, err = f(ctx)
		return err
	})
	return ret, v.Done(callErr)
}

func (ev *ActionEvent) Log(ctx context.Context) {
	sink := SinkFrom(ctx)
	if sink == nil {
		return
	}

	ev.initMissing()
	if ev.data.Started.IsZero() {
		ev.data.Started = ev.data.Created
	}
	ev.data.State = ActionInstant
	sink.Instant(&ev.data)
}

func makeProto(data *EventData, at *EventAttachments) *protocol.Task {
	p := &protocol.Task{
		Id:                 data.ActionID,
		Name:               data.Name,
		HumanReadableLabel: data.HumanReadable,
		CreatedTs:          data.Started.UnixNano(),
		Scope:              data.Scope.PackageNamesAsString(),
	}

	if data.State == ActionDone {
		p.CompletedTs = data.Completed.UnixNano()
		if data.Err != nil {
			p.ErrorMessage = data.Err.Error()
		}
	}

	if at != nil {
		at.mu.Lock()
		for _, name := range at.insertionOrder {
			p.Output = append(p.Output, &protocol.Task_Output{
				Name:        at.buffers[name.computed].name,
				ContentType: at.buffers[name.computed].contentType,
			})
		}
		at.mu.Unlock()
	}

	return p
}

func makeDebugProto(data *EventData, at *EventAttachments) *protocol.StoredTask {
	p := &protocol.StoredTask{
		Id:                 data.ActionID,
		Name:               data.Name,
		HumanReadableLabel: data.HumanReadable,
		CreatedTs:          data.Started.UnixNano(),
		Scope:              data.Scope.PackageNamesAsString(),
	}

	if data.State == ActionDone {
		p.CompletedTs = data.Completed.UnixNano()
		if data.Err != nil {
			p.ErrorMessage = data.Err.Error()
		}
	}

	for _, x := range data.Arguments {
		serialized, err := json.MarshalIndent(x.Msg, "", "  ")
		if err != nil {
			serialized = []byte("{\"error\": \"failed to serialize\"}")
		}

		p.Argument = append(p.Argument, &protocol.StoredTask_Value{
			Key:       x.Name,
			JsonValue: string(serialized),
		})
	}

	if at != nil {
		at.mu.Lock()
		if at.ResultData.Items != nil {
			for _, x := range at.Items {
				serialized, err := json.MarshalIndent(x.Msg, "", "  ")
				if err != nil {
					serialized = []byte("{\"error\": \"failed to serialize\"}")
				}

				p.Result = append(p.Result, &protocol.StoredTask_Value{
					Key:       x.Name,
					JsonValue: string(serialized),
				})
			}
		}

		for k, name := range at.insertionOrder {
			p.Output = append(p.Output, &protocol.StoredTask_Output{
				Id:          fmt.Sprintf("%d", k),
				Name:        at.buffers[name.computed].name,
				ContentType: at.buffers[name.computed].contentType,
			})
		}
		at.mu.Unlock()
	}

	return p
}

func (af *RunningAction) ID() string                     { return af.Data.ActionID }
func (af *RunningAction) Proto() *protocol.Task          { return makeProto(&af.Data, af.attachments) }
func (af *RunningAction) Attachments() *EventAttachments { return af.attachments }

func startSpan(ctx context.Context, data EventData) trace.Span {
	name := data.Name
	if data.Category != "" {
		name = data.Category + "::" + name
	}
	_, span := otel.Tracer("fn").Start(ctx, name)

	if span.IsRecording() {
		span.SetAttributes(attribute.String("actionID", data.ActionID))
		if data.AnchorID != "" {
			span.SetAttributes(attribute.String("anchorID", data.AnchorID))
		}

		for _, arg := range data.Arguments {
			// The stored value is serialized in a best-effort way.
			be, _ := json.MarshalIndent(arg.Msg, "", "  ")
			span.SetAttributes(attribute.String("arg."+arg.Name, string(be)))
		}

		if data.Scope.Len() > 0 {
			span.SetAttributes(attribute.StringSlice("scope", data.Scope.PackageNamesAsString()))
		}
	}

	return span
}

func endSpan(span trace.Span, r ResultData, completed time.Time) {
	for _, arg := range r.Items {
		// The stored value is serialized in a best-effort way.
		be, _ := json.MarshalIndent(arg.Msg, "", "  ")
		span.SetAttributes(attribute.String("result."+arg.Name, string(be)))
	}
	// XXX we should pass along the completed timeline, but we're sometimes seeing arbitrary values
	// into the future.
	// span.End(trace.WithTimestamp(completed))
	span.End()
}

func (af *RunningAction) markStarted(ctx context.Context) {
	if af.Data.Started.IsZero() {
		af.Data.Started = af.Data.Created
	}
	af.Data.State = ActionRunning
	af.sink.Started(af)
}

func (af *RunningAction) CustomDone(t time.Time, err error) bool {
	if af != nil && af.Data.State != ActionDone {
		if af.onDone != nil {
			// Call onDone first, so it's execution time is accounted for to the action time.
			af.onDone(af.Proto())
		}

		if af.Data.Completed.IsZero() {
			af.Data.Completed = t
		}

		af.Data.State = ActionDone
		af.Data.Err = err

		if af.attachments != nil {
			af.attachments.seal()
		}

		if af.span != nil {
			if err != nil {
				af.span.SetStatus(codes.Error, err.Error())
			}

			endSpan(af.span, af.attachments.ResultData, af.Data.Completed)
		}

		af.sink.Done(af)

		if ActionStorer != nil {
			ActionStorer.Store(af)
		}

		return true
	}

	return false
}

func (af *RunningAction) Call(ctx context.Context, f func(context.Context) error) error {
	ctx = context.WithValue(ctx, _actionKey, af)

	if af.span != nil {
		return f(trace.ContextWithSpan(ctx, af.span))
	}

	return f(ctx)
}

func (af *RunningAction) Done(err error) error {
	// XXX serialize additional error data.
	af.CustomDone(time.Now(), err)
	return err
}

type OutputName struct{ name, contentType, computed string }

func Output(name string, contentType string) OutputName {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return OutputName{name, contentType, name + "[" + contentType + "]"}
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

	ev.insertionOrder = append(ev.insertionOrder, name)
	ev.buffers[name.computed] = attachedBuffer{
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

	msg, err := common.Serialize(body)
	if err != nil {
		return fnerrors.BadInputError("failed to serialize payload: %w", err)
	}

	bytes, err := common.SerializeToBytes(msg)
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

func (ev *EventAttachments) ensureOutput(name OutputName, addIfMissing bool) (io.Writer, bool) {
	if ev == nil {
		return io.Discard, false
	}

	ev.mu.Lock()
	defer ev.mu.Unlock()

	if !addIfMissing && ev.buffers == nil {
		return io.Discard, false
	}

	ev.init()

	added := false
	if _, ok := ev.buffers[name.computed]; !ok {
		if !addIfMissing {
			return io.Discard, false
		}

		ev.buffers[name.computed] = attachedBuffer{
			buffer:      syncbuffer.NewByteBuffer(),
			name:        name.name,
			contentType: name.contentType,
		}
		ev.insertionOrder = append(ev.insertionOrder, name)
		added = true
	}

	return ev.buffers[name.computed].buffer.Writer(), added
}

func (ev *EventAttachments) Output(name OutputName) io.Writer {
	w, added := ev.ensureOutput(name, true)
	if added {
		ev.sink.AttachmentsUpdated(ev.actionID, nil)
	}
	return w
}

func (ev *EventAttachments) ActionID() string {
	if ev == nil {
		return ""
	}
	return ev.actionID
}

func WithSink(ctx context.Context, sink ActionSink) context.Context {
	return context.WithValue(ctx, _sinkKey, sink)
}

func SinkFrom(ctx context.Context) ActionSink {
	sink := ctx.Value(_sinkKey)
	if sink == nil {
		return nil
	}
	return sink.(ActionSink)
}

func Attachments(ctx context.Context) *EventAttachments {
	v := ctx.Value(_actionKey)
	if v == nil {
		return nil
	}
	return v.(*RunningAction).Attachments()
}

func NameOf(ev *ActionEvent) (string, string) {
	return ev.data.Name, ev.data.HumanReadable
}
