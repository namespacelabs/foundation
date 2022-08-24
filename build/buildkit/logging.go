// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/opencontainers/go-digest"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var TaskOutputBuildkitJsonLog = tasks.Output("buildkit.json", "application/json+fn.buildkit")

var UsePlaintextLogging = false

type jsonEvent struct {
	SessionID string              `json:"s"`
	Started   *time.Time          `json:"started,omitempty"`
	Completed *time.Time          `json:"completed,omitempty"`
	Event     *client.SolveStatus `json:"e,omitempty"`
}

func setupOutput(ctx context.Context, logid, sid string, eg executor.ExecutorLike, parentCh chan *client.SolveStatus) {
	attachments := tasks.Attachments(ctx)
	outText := attachments.Output(tasks.TaskOutputTextLog)
	outJSON := attachments.Output(TaskOutputBuildkitJsonLog)

	console.GetErrContext(ctx).AddLog(tasks.TaskOutputTextLog)

	writers := []io.Writer{outText}
	jsonWriters := []io.Writer{outJSON}

	channelCount := len(writers) + len(jsonWriters)
	if UsePlaintextLogging {
		writers = append(writers, console.Output(ctx, console.MakeConsoleName(logid, sid, "")))
		channelCount++
	} else {
		channelCount++
	}

	chs := make([]chan *client.SolveStatus, channelCount)
	for k := range chs {
		chs[k] = make(chan *client.SolveStatus)
	}

	eg.Go(func(_ context.Context) error {
		// Copy output to N consoles.
		for v := range parentCh {
			for _, child := range chs {
				child <- v
			}
		}

		for _, child := range chs {
			close(child)
		}

		return nil
	})

	for k := range writers {
		k := k // Capture k
		eg.Go(func(ctx context.Context) error {
			// Don't propagate context cancelation, rather let the channel management above
			// decide when we should bail out. Without this, DisplaySolveStatus may decide to
			// return because it observed a cancellation. But it's channel is not closed. Which
			// leads to writes above blocking, as there's no consumer. If those writes block,
			// then buildkit's Solve can't return, as it's waiting to push a status update. And
			// that will lead to it never returning from a cancelation (8h+ were spent on this issue).
			_, err := progressui.DisplaySolveStatus(context.Background(), "", nil,
				&timestampPrefixWriter{writers[k], time.Now, true}, chs[k])
			if err != nil {
				return fnerrors.InternalError("buildkit progress ui failed: %w", err)
			}

			return nil
		})
	}

	for k := range jsonWriters {
		k := k // Capture k
		eg.Go(func(ctx context.Context) error {
			ch := chs[k+len(writers)]

			now := time.Now()
			if err := pushJsonEvent(jsonWriters[k], jsonEvent{SessionID: sid, Started: &now}); err != nil {
				return err
			}

			for event := range ch {
				if err := pushJsonEvent(jsonWriters[k], jsonEvent{SessionID: sid, Event: event}); err != nil {
					return err
				}
			}

			end := time.Now()
			if err := pushJsonEvent(jsonWriters[k], jsonEvent{SessionID: sid, Completed: &end}); err != nil {
				return err
			}

			return nil
		})
	}

	if UsePlaintextLogging {
		return // Skip the next block.
	}

	eg.Go(func(ctx context.Context) error {
		ch := chs[len(chs)-1]

		running := map[string]*vertexState{}
		streams := map[string]io.Writer{}

		for event := range ch {
			for _, vertex := range event.Vertexes {
				vid := vertex.Digest.Encoded()
				name := vertex.Name

				if vertex.Cached {
					tasks.Action(name).Category("buildkit").Arg("cached", true).Log(ctx)
					continue
				}

				existing := running[vid]
				if vertex.Started != nil && vertex.Completed == nil {
					existing = &vertexState{
						action:   tasks.Action(name).Category("buildkit").StartTimestamp(*vertex.Started).Start(ctx),
						statuses: map[string]*tasks.RunningAction{},
					}
					running[vid] = existing
				}

				if vertex.Completed != nil && existing != nil {
					var err error
					if vertex.Error != "" {
						err = fnerrors.New(vertex.Error)
						// TODO mark a buffer storing the error message.
					}

					existing.customDone(*vertex.Completed, err)
					delete(running, vid)
				}
			}

			for _, status := range event.Statuses {
				vid := status.Vertex.Encoded()

				parent := running[vid]
				if parent == nil {
					// Should never happen.
					continue
				}

				sid := status.ID

				existing := parent.statuses[sid]
				if status.Started != nil {
					if existing == nil {
						existing = tasks.Action(sid).Category("buildkit").Parent(parent.action.ID()).StartTimestamp(*status.Started).Start(ctx)
						parent.statuses[sid] = existing
						// XXX implement progress tracking, buildkit will send updated `Current` counts.
					}
				}

				if status.Completed != nil && existing != nil {
					existing.CustomDone(*status.Completed, nil)
					delete(running, sid)
				}
			}

			for _, log := range event.Logs {
				key := fmt.Sprintf("%s/%d", log.Vertex, log.Stream)

				if streams[key] == nil {
					name := consoleName(logid, log.Vertex, log.Stream)
					outputName := tasks.Output(name, "text/plain")
					output := tasks.Attachments(ctx).Output(outputName)
					streams[key] = io.MultiWriter(output, console.Output(ctx, name))
					console.GetErrContext(ctx).AddLog(outputName)
				}

				_, _ = streams[key].Write(log.Data)
			}
		}

		for _, ra := range running {
			ra.customDone(time.Now(), fnerrors.New("never finished"))
		}

		return nil
	})
}

func consoleName(logid string, d digest.Digest, streamNum int) string {
	if logid == "" {
		logid = "buildkit"
	}

	return console.MakeConsoleName(logid, d.Hex(), streamName(streamNum))
}

func streamName(streamNum int) string {
	// https://github.com/moby/buildkit/blob/08497dafaff7b99f4e1780f17475e327c940b3f6/util/progress/logs/logs.go#L25-L26
	switch streamNum {
	case 1, 2:
		return ""
	default:
		return fmt.Sprintf(":%d", streamNum)
	}
}

type vertexState struct {
	action   *tasks.RunningAction
	statuses map[string]*tasks.RunningAction
}

func (vs *vertexState) customDone(t time.Time, err error) {
	for _, st := range vs.statuses {
		st.CustomDone(t, err)
	}

	vs.action.CustomDone(t, err)
}

func pushJsonEvent(w io.Writer, ev jsonEvent) error {
	p, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	// Make a space for a newline, easier to parse.
	pline := make([]byte, len(p)+1)
	copy(pline, p)
	pline[len(p)] = '\n'

	if _, err := w.Write(pline); err != nil {
		return err
	}

	return nil
}

type timestampPrefixWriter struct {
	w                io.Writer
	clock            func() time.Time
	pendingTimestamp bool
}

func (tsw *timestampPrefixWriter) Write(p []byte) (int, error) {
	for len(p) > 0 {
		if tsw.pendingTimestamp {
			fmt.Fprintf(tsw.w, "%s ", tsw.clock().Format(time.RFC3339Nano))
			tsw.pendingTimestamp = false
		}

		k := bytes.Index(p, []byte("\n"))
		if k < 0 {
			_, _ = tsw.w.Write(p)
			break
		} else {
			_, _ = tsw.w.Write(p[:(k + 1)])
			p = p[k+1:]
			tsw.pendingTimestamp = true
		}
	}

	return len(p), nil
}
