// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"fmt"

	"github.com/rs/zerolog"
	"namespacelabs.dev/foundation/internal/console/common"
)

func NewLoggerSink(logger *zerolog.Logger) ActionSink { return &sinkLogger{logger} }

type sinkLogger struct{ logger *zerolog.Logger }

func (sl *sinkLogger) start(ev EventData, withArgs bool) *zerolog.Event {
	e := sl.logger.Info().Str("action_id", ev.ActionID).Str("name", ev.Name).Int("log_level", ev.Level)
	if ev.ParentID != "" {
		e = e.Str("parent_id", ev.ParentID)
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

func (sl *sinkLogger) Waiting(ra *RunningAction) {
	// Do nothing.
}

func (sl *sinkLogger) Started(ra *RunningAction) {
	sl.start(ra.Data, true).Msg("start")
}

func (sl *sinkLogger) Done(ra *RunningAction) {
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

func (sl *sinkLogger) Instant(ev *EventData) {
	sl.start(*ev, true).Msg(ev.Name)
}

func (sl *sinkLogger) AttachmentsUpdated(string, *ResultData) { /* nothing to do */ }
