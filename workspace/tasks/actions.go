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
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	AttachmentsUpdated(string, *resultData)
}

type actionState string

const (
	actionCreated = "fn.action.created"
	actionWaiting = "fn.action.waiting"
	actionRunning = "fn.action.running"
	actionDone    = "fn.action.done"
	actionInstant = "fn.action.instant"
)

func (a actionState) IsRunning() bool { return a == actionWaiting || a == actionRunning }
func (a actionState) IsDone() bool    { return a == actionDone || a == actionInstant }

type OnDoneFunc func(*protocol.Task)

type WellKnown string

const (
	WkAction = "action"
	WkModule = "module"
)

type EventData struct {
	actionID      string
	parentID      string
	anchorID      string // This action represents "waiting" on the action represented by `anchorID`.
	state         actionState
	name          string
	humanReadable string // If not set, name is used.
	category      string
	created       time.Time
	started       time.Time
	completed     time.Time
	arguments     []actionArgument
	scope         schema.PackageList
	level         int
	indefinite    bool
	err           error
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

type resultData struct {
	items    []*actionArgument
	progress ActionProgress
}

type EventAttachments struct {
	actionID string
	sink     ActionSink

	mu sync.Mutex // Protects below.
	resultData
	// For the time being we just keep everything in memory for simplicity.
	buffers        map[string]attachedBuffer
	insertionOrder []OutputName
}

type RunningAction struct {
	sink        ActionSink
	data        EventData
	span        trace.Span
	attachments *EventAttachments
	progress    ActionProgress
	onDone      OnDoneFunc
}

type actionArgument struct {
	Name string
	msg  interface{}
}

func serialize(msg interface{}) (interface{}, error) {
	if s, ok := msg.(SerializableArgument); ok {
		return s.SerializeAsJSON()
	}
	if s, ok := msg.(fmt.Stringer); ok {
		return s.String(), nil
	}
	return msg, nil
}

func serializeToBytes(msg interface{}) ([]byte, error) {
	return json.Marshal(msg)
}

type SerializableArgument interface {
	SerializeAsJSON() (interface{}, error)
}

func allocEvent() *ActionEvent {
	return &ActionEvent{}
}

func Action(name string) *ActionEvent {
	ev := allocEvent()
	ev.data.name = name
	ev.data.state = actionCreated
	return ev
}

func (ev *ActionEvent) HumanReadablef(label string, args ...interface{}) *ActionEvent {
	if len(args) == 0 {
		ev.data.humanReadable = label
	} else {
		ev.data.humanReadable = fmt.Sprintf(label, args...)
	}
	return ev
}

func (ev *ActionEvent) Serializable(name string, msg SerializableArgument) *ActionEvent {
	ev.data.arguments = append(ev.data.arguments, actionArgument{Name: name, msg: msg})
	return ev
}

func (ev *ActionEvent) OnDone(f OnDoneFunc) *ActionEvent {
	ev.onDone = f
	return ev
}

func (ev *ActionEvent) ID(id string) *ActionEvent {
	ev.data.actionID = id
	return ev
}

func (ev *ActionEvent) Anchor(id string) *ActionEvent {
	ev.data.anchorID = id
	return ev
}

func (ev *ActionEvent) StartTimestamp(ts time.Time) *ActionEvent {
	ev.data.started = ts
	return ev
}

func (ev *ActionEvent) Category(category string) *ActionEvent {
	ev.data.category = category
	return ev
}

func (ev *ActionEvent) Parent(tid string) *ActionEvent {
	ev.data.parentID = tid
	return ev
}

func (ev *ActionEvent) Scope(pkgs ...schema.PackageName) *ActionEvent {
	ev.data.scope.AddMultiple(pkgs...)
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
	if ev.data.actionID == "" {
		ev.data.actionID = NewActionID()
	}
	ev.data.created = time.Now()
}

// Sets the level for this action (by default it's zero). The lower the level, the higher the importance.
func (ev *ActionEvent) LogLevel(level int) *ActionEvent {
	ev.data.level = level
	return ev
}

func (ev *ActionEvent) Arg(name string, msg interface{}) *ActionEvent {
	ev.data.arguments = append(ev.data.arguments, actionArgument{Name: name, msg: msg})
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
	ev.data.indefinite = true
	return ev
}

func (ev *ActionEvent) Clone(makeName func(string) string) *ActionEvent {
	copy := &ActionEvent{
		data: ev.data,
	}

	name := copy.data.name
	if copy.data.category != "" {
		name = copy.data.category + "::" + name
		copy.data.category = ""
	}

	copy.data.name = makeName(name)
	return copy
}

func (ev *ActionEvent) toAction(ctx context.Context, state actionState) *RunningAction {
	sink := SinkFrom(ctx)
	if sink == nil {
		return nil
	}

	var parent *RunningAction
	action := ctx.Value(_actionKey)
	if action != nil {
		parent = action.(*RunningAction)
		ev.data.parentID = parent.data.actionID
	}

	ev.initMissing()

	ev.data.state = state

	span := startSpan(ctx, ev.data)

	return &RunningAction{
		sink:        sink,
		data:        ev.data,
		span:        span,
		progress:    ev.progress,
		attachments: &EventAttachments{actionID: ev.data.actionID, sink: sink},
		onDone:      ev.onDone,
	}
}

func (ev *ActionEvent) Start(ctx context.Context) *RunningAction {
	ra := ev.toAction(ctx, actionRunning)
	ra.markStarted(ctx)
	return ra
}

type RunOptions struct {
	Wait func(context.Context, map[WellKnown]string) (bool, error)
	Run  func(context.Context) error
}

func (ev *ActionEvent) CheckCacheRun(ctx context.Context, options RunOptions) error {
	ra := ev.toAction(ctx, actionWaiting)
	ra.sink.Waiting(ra)

	var wasCached bool
	err := ra.Call(ctx, func(ctx context.Context) error {
		if _, ok := ev.wellKnown[WkAction]; !ok {
			if ev.wellKnown == nil {
				ev.wellKnown = map[WellKnown]string{}
			}
			ev.wellKnown[WkAction] = ev.data.name
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
		ra.data.arguments = append(ra.data.arguments, actionArgument{Name: "cached", msg: true})
		return ra.Done(nil)
	}

	// Our data model implies that the caller always owns data; and sinks should perform copies.
	ra.data.started = time.Now()
	ra.markStarted(ctx)

	return ra.Done(ra.Call(ctx, options.Run))
}

func (ev *ActionEvent) Run(ctx context.Context, f func(context.Context) error) error {
	v := ev.Start(ctx)
	return v.Done(v.Call(ctx, f))
}

func (ev *ActionEvent) Log(ctx context.Context) {
	sink := SinkFrom(ctx)
	if sink == nil {
		return
	}

	ev.initMissing()
	if ev.data.started.IsZero() {
		ev.data.started = ev.data.created
	}
	ev.data.state = actionInstant
	sink.Instant(&ev.data)
}

func makeProto(data *EventData, at *EventAttachments) *protocol.Task {
	p := &protocol.Task{
		Id:                 data.actionID,
		Name:               data.name,
		HumanReadableLabel: data.humanReadable,
		CreatedTs:          data.started.UnixNano(),
		Scope:              data.scope.PackageNamesAsString(),
	}

	if data.state == actionDone {
		p.CompletedTs = data.completed.UnixNano()
		if data.err != nil {
			p.ErrorMessage = data.err.Error()
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
		Id:                 data.actionID,
		Name:               data.name,
		HumanReadableLabel: data.humanReadable,
		CreatedTs:          data.started.UnixNano(),
		Scope:              data.scope.PackageNamesAsString(),
	}

	if data.state == actionDone {
		p.CompletedTs = data.completed.UnixNano()
		if data.err != nil {
			p.ErrorMessage = data.err.Error()
		}
	}

	for _, x := range data.arguments {
		serialized, err := json.MarshalIndent(x.msg, "", "  ")
		if err != nil {
			serialized = []byte("{\"error\": \"failed to serialize\"}")
		}

		p.Argument = append(p.Argument, &protocol.StoredTask_Value{
			Key:       x.Name,
			JsonValue: string(serialized),
		})
	}

	if at != nil {
		if at.resultData.items != nil {
			for _, x := range at.items {
				serialized, err := json.MarshalIndent(x.msg, "", "  ")
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
	}

	return p
}

func (af *RunningAction) ID() string                     { return af.data.actionID }
func (af *RunningAction) Proto() *protocol.Task          { return makeProto(&af.data, af.attachments) }
func (af *RunningAction) Attachments() *EventAttachments { return af.attachments }

func startSpan(ctx context.Context, data EventData) trace.Span {
	name := data.name
	if data.category != "" {
		name = data.category + "::" + name
	}
	_, span := otel.Tracer("fn").Start(ctx, name)

	if span.IsRecording() {
		span.SetAttributes(attribute.String("actionID", data.actionID))
		if data.anchorID != "" {
			span.SetAttributes(attribute.String("anchorID", data.anchorID))
		}

		for _, arg := range data.arguments {
			// The stored value is serialized in a best-effort way.
			be, _ := json.MarshalIndent(arg.msg, "", "  ")
			span.SetAttributes(attribute.String("arg."+arg.Name, string(be)))
		}

		if data.scope.Len() > 0 {
			span.SetAttributes(attribute.StringSlice("scope", data.scope.PackageNamesAsString()))
		}
	}

	return span
}

func endSpan(span trace.Span, r resultData, completed time.Time) {
	for _, arg := range r.items {
		// The stored value is serialized in a best-effort way.
		be, _ := json.MarshalIndent(arg.msg, "", "  ")
		span.SetAttributes(attribute.String("result."+arg.Name, string(be)))
	}
	// XXX we should pass along the completed timeline, but we're sometimes seeing arbitrary values
	// into the future.
	// span.End(trace.WithTimestamp(completed))
	span.End()
}

func (af *RunningAction) markStarted(ctx context.Context) {
	if af.data.started.IsZero() {
		af.data.started = af.data.created
	}
	af.data.state = actionRunning
	af.sink.Started(af)
}

func (af *RunningAction) CustomDone(t time.Time, err error) bool {
	if af != nil && af.data.state != actionDone {
		if af.onDone != nil {
			// Call onDone first, so it's execution time is accounted for to the action time.
			af.onDone(af.Proto())
		}

		if af.data.completed.IsZero() {
			af.data.completed = t
		}

		af.data.state = actionDone
		af.data.err = err

		if af.attachments != nil {
			af.attachments.seal()
		}

		if af.span != nil {
			if err != nil {
				af.span.SetStatus(codes.Error, err.Error())
			}

			endSpan(af.span, af.attachments.resultData, af.data.completed)
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

func (ev *EventAttachments) AttachSerializable(name, modifier string, body interface{}) {
	if ev == nil {
		panic("no running action")
	}

	msg, err := serialize(body)
	if err != nil {
		panic(fmt.Sprintf("failed to serialize payload: %v", err))
	}

	bytes, err := serializeToBytes(msg)
	if err != nil {
		panic(fmt.Sprintf("failed to serialize payload to bytes: %v", err))
	}

	contentType := "application/json"
	if modifier != "" {
		contentType += "+" + modifier
	}

	ev.Attach(Output(name, contentType), bytes)
}

func (ev *EventAttachments) addResult(key string, msg interface{}) resultData {
	ev.mu.Lock()
	defer ev.mu.Unlock()

	for _, item := range ev.items {
		if item.Name == key {
			item.msg = msg
			return ev.resultData
		}
	}

	// Not found, add a new result.
	ev.items = append(ev.items, &actionArgument{key, msg})
	return ev.resultData
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
		ev.progress = p
		copy := ev.resultData
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
	return ev.data.name, ev.data.humanReadable
}
