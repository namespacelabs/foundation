// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	statuscodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace/tasks/protocol"
	"namespacelabs.dev/go-ids"
)

var ActionStorer *Storer = nil

type ActionSink interface {
	Waiting(*RunningAction)
	Started(*RunningAction)
	Done(*RunningAction)
	Instant(*EventData)
	AttachmentsUpdated(ActionID, *ResultData)
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

type ActionID string

func (a ActionID) String() string { return string(a) }

const (
	WkAction   = "action"
	WkCategory = "category"
	WkModule   = "module"
	WkRuntime  = "tool-runtime"
)

type EventData struct {
	ActionID       ActionID
	ParentID       ActionID
	AnchorID       ActionID // This action represents "waiting" on the action represented by `anchorID`.
	SpanID         string
	State          ActionState
	Name           string
	HumanReadable  string // If not set, name is used.
	Category       string
	Created        time.Time
	Started        time.Time
	Completed      time.Time
	Arguments      []ActionArgument
	Scope          schema.PackageList
	Level          int
	HasPrivateData bool
	Indefinite     bool
	Err            error
}

type ActionEvent struct {
	data      EventData
	progress  ActionProgress
	onDone    OnDoneFunc
	wellKnown map[WellKnown]string
}

type ResultData struct {
	Items    []*ActionArgument
	Progress ActionProgress
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

func (ev *ActionEvent) OnDone(f OnDoneFunc) *ActionEvent {
	ev.onDone = f
	return ev
}

func (ev *ActionEvent) ID(id ActionID) *ActionEvent {
	ev.data.ActionID = id
	return ev
}

func (ev *ActionEvent) Anchor(id ActionID) *ActionEvent {
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

func (ev *ActionEvent) Parent(tid ActionID) *ActionEvent {
	ev.data.ParentID = tid
	return ev
}

func (ev *ActionEvent) Scope(pkgs ...schema.PackageName) *ActionEvent {
	ev.data.Scope.AddMultiple(pkgs...)
	return ev
}

func (ev *ActionEvent) IncludesPrivateData() *ActionEvent {
	ev.data.HasPrivateData = true
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

func NewActionID() ActionID { return ActionID(ids.NewRandomBase62ID(16)) }

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

func (ev *ActionEvent) Str(name string, msg fmt.Stringer) *ActionEvent {
	ev.data.Arguments = append(ev.data.Arguments, ActionArgument{Name: name, Msg: msg.String()})
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
	if ra != nil {
		ra.markStarted(ctx)
	}
	return ra
}

type RunOpts struct {
	// If Wait returns true, then the action is considered to be cached, and Run is skipped.
	Wait func(context.Context) (bool, error)
	Run  func(context.Context) error
}

type handledPanic struct {
	v any
}

// Separate function to make the stack trace more readable.
func panicHandler(ctx context.Context) {
	r := recover()
	if r == nil {
		return
	}

	if _, ok := r.(handledPanic); ok {
		// bubble up panic.
		panic(r)
	}

	// Capture the stack on panic.
	_ = ActionStorer.WriteRuntimeStack(ctx, debug.Stack())

	// Ensure that we always have an audit trail.
	_ = ActionStorer.Flush(ctx)

	// Mark panic as handled and bubble it up.
	panic(handledPanic{v: r})
}

func (ev *ActionEvent) RunWithOpts(ctx context.Context, opts RunOpts) error {
	defer panicHandler(ctx)

	ra := ev.toAction(ctx, ActionWaiting)
	ra.sink.Waiting(ra)

	var wasCached bool
	var releaseLease func()
	err := ra.Call(ctx, func(ctx context.Context) error {
		if _, ok := ev.wellKnown[WkAction]; !ok {
			if ev.wellKnown == nil {
				ev.wellKnown = map[WellKnown]string{}
			}
			ev.wellKnown[WkAction] = ev.data.Name
		}

		if opts.Wait != nil {
			cached, err := opts.Wait(ctx)
			if err != nil {
				return err
			}
			wasCached = cached
			if cached {
				// Don't try to acquire a lease.
				return nil
			}
		}

		// Classify the wait for lease time as "wait time".
		var err error
		releaseLease, err = throttlerFromContext(ctx).AcquireLease(ctx, ev.wellKnown)
		return err
	})
	if err != nil {
		return ra.Done(err)
	}

	// Only record a Starting event if we had to wait.
	if (opts.Wait != nil || releaseLease != nil) && ra.span != nil && ra.span.IsRecording() {
		ra.span.AddEvent("starting", trace.WithAttributes(attribute.Bool("cached", wasCached)))
		ra.Data.SpanID = ra.span.SpanContext().SpanID().String()
	}

	if wasCached {
		ra.Data.Arguments = append(ra.Data.Arguments, ActionArgument{Name: "cached", Msg: true})
		return ra.Done(nil)
	}

	if releaseLease != nil {
		defer releaseLease()
	}

	// Our data model implies that the caller always owns data; and sinks should perform copies.
	ra.Data.Started = time.Now()
	ra.markStarted(ctx)

	return ra.Done(ra.Call(ctx, opts.Run))
}

func (ev *ActionEvent) Run(ctx context.Context, f func(context.Context) error) error {
	return ev.RunWithOpts(ctx, RunOpts{Run: f})
}

func Return[V any](ctx context.Context, ev *ActionEvent, f func(context.Context) (V, error)) (V, error) {
	var ret V
	err := ev.RunWithOpts(ctx, RunOpts{Run: func(ctx context.Context) error {
		var err error
		ret, err = f(ctx)
		return err
	}})
	return ret, err
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
		Id:                 data.ActionID.String(),
		Name:               data.Name,
		HumanReadableLabel: data.HumanReadable,
		CreatedTs:          data.Created.UnixNano(),
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
				Id:          at.buffers[name.computed].id,
				Name:        at.buffers[name.computed].name,
				ContentType: at.buffers[name.computed].contentType,
			})
		}
		at.mu.Unlock()
	}

	return p
}

func makeStoreProto(data *EventData, at *EventAttachments) *storage.StoredTask {
	p := &storage.StoredTask{
		Id:                 data.ActionID.String(),
		ParentId:           data.ParentID.String(),
		AnchorId:           data.AnchorID.String(),
		SpanId:             data.SpanID,
		Name:               data.Name,
		Category:           data.Category,
		HumanReadableLabel: data.HumanReadable,
		CreatedTs:          data.Created.UnixNano(),
		Scope:              data.Scope.PackageNamesAsString(),
		LogLevel:           int32(data.Level),
	}

	if data.State == ActionDone {
		p.CompletedTs = data.Completed.UnixNano()
		p.RelStartedTs = data.Started.UnixNano() - data.Created.UnixNano()
		p.RelCompletedTs = data.Completed.UnixNano() - data.Created.UnixNano()

		if data.Err != nil {
			st, _ := status.FromError(data.Err)
			p.ErrorCode = int32(st.Code())
			p.ErrorMessage = st.Message()
			p.ErrorDetails = st.Proto().Details

			if errors.Is(data.Err, context.Canceled) {
				p.ErrorCode = int32(statuscodes.Canceled)
			}
		}
	}

	for _, x := range data.Arguments {
		p.Argument = append(p.Argument, &storage.StoredTask_Value{
			Key:       x.Name,
			JsonValue: serialize(x.Msg, data.HasPrivateData),
		})
	}

	if at != nil {
		at.mu.Lock()
		if at.ResultData.Items != nil {
			for _, x := range at.Items {
				if x.Name == "cached" && x.Msg == true {
					p.Cached = true
				} else {
					p.Result = append(p.Result, &storage.StoredTask_Value{
						Key:       x.Name,
						JsonValue: serialize(x.Msg, data.HasPrivateData),
					})
				}
			}
		}

		if !data.HasPrivateData {
			for _, name := range at.insertionOrder {
				p.Output = append(p.Output, &storage.StoredTask_Output{
					Id:          at.buffers[name.computed].id,
					Name:        at.buffers[name.computed].name,
					ContentType: at.buffers[name.computed].contentType,
				})
			}
		}
		at.mu.Unlock()
	}

	return p
}

func serialize(msg interface{}, hasPrivateData bool) string {
	if hasPrivateData {
		return "$redacted"
	}

	serialized, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return "{\"error\": \"failed to serialize\"}"
	}

	return string(serialized)
}

func (af *RunningAction) ID() ActionID                   { return af.Data.ActionID }
func (af *RunningAction) Proto() *protocol.Task          { return makeProto(&af.Data, af.attachments) }
func (af *RunningAction) Attachments() *EventAttachments { return af.attachments }

func startSpan(ctx context.Context, data EventData) trace.Span {
	name := data.Name
	if data.Category != "" {
		name = data.Category + "::" + name
	}
	_, span := otel.Tracer("fn").Start(ctx, name)

	if span.IsRecording() {
		span.SetAttributes(attribute.String("actionID", data.ActionID.String()))
		if data.AnchorID != "" {
			span.SetAttributes(attribute.String("anchorID", data.AnchorID.String()))
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

		// It's fundamental that seal() is called above before Store(); else
		// we'll spin forever trying to consume the open buffers.
		if ActionStorer != nil {
			ActionStorer.Store(af)
		}

		return true
	}

	return false
}

func (af *RunningAction) Call(ctx context.Context, f func(context.Context) error) error {
	if af != nil {
		ctx = context.WithValue(ctx, _actionKey, af)

		if af.span != nil {
			return f(trace.ContextWithSpan(ctx, af.span))
		}
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
