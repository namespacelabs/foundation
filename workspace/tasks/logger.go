// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"fmt"

	"github.com/rs/zerolog"
	"namespacelabs.dev/foundation/internal/console/common"
)

func NewJsonLoggerSink(logger *zerolog.Logger) ActionSink { return &jsonLogger{logger} }

type jsonLogger struct{ logger *zerolog.Logger }

func (sl *jsonLogger) start(ev EventData, withArgs bool) *zerolog.Event {
	e := sl.logger.Info().Str("action_id", ev.ActionID.String()).Str("name", ev.Name).Int("log_level", ev.Level)
	if !ev.ParentID.IsEmpty() {
		e = e.Str("parent_id", ev.ParentID.String())
	}
	if withArgs {
		if ev.Scope.Len() > 0 {
			e = e.Strs("scope", ev.Scope.PackageNamesAsString())
		}

		for _, arg := range ev.Arguments {
			res, err := common.Serialize(arg.Msg)
			if err != nil {
				e = e.Interface(arg.Name, fmt.Sprintf("failed to serialize: %v", err))
			} else {
				e = e.Interface(arg.Name, res)
			}
		}
	}
	return e
}

func (sl *jsonLogger) Waiting(ra *RunningAction) {
	// Do nothing.
}

func (sl *jsonLogger) Started(ra *RunningAction) {
	sl.start(ra.Data, true).Msg("start")
}

func (sl *jsonLogger) Done(ra *RunningAction) {
	ev := sl.start(ra.Data, true)
	if ra.Data.Err != nil {
		t := ErrorType(ra.Data.Err)
		switch t {
		case ErrTypeIsCancelled, ErrTypeIsDependencyFailed:
			ev.Msg(string(t))
			return
		default:
			ev = ev.Stack().Err(ra.Data.Err)
		}
	}
	ev.Dur("took", ra.Data.Completed.Sub(ra.Data.Started)).Msg("done")
}

func (sl *jsonLogger) Instant(ev *EventData) {
	sl.start(*ev, true).Msg(ev.Name)
}

func (sl *jsonLogger) AttachmentsUpdated(ActionID, *ResultData) { /* nothing to do */ }
