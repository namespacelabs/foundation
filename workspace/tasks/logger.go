// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"fmt"

	"github.com/rs/zerolog"
)

func NewLoggerSink(logger *zerolog.Logger) ActionSink { return &sinkLogger{logger} }

type sinkLogger struct{ logger *zerolog.Logger }

func (sl *sinkLogger) start(ev EventData, withArgs bool) *zerolog.Event {
	e := sl.logger.Info().Str("action_id", ev.actionID).Str("name", ev.name).Int("log_level", ev.level)
	if ev.parentID != "" {
		e = e.Str("parent_id", ev.parentID)
	}
	if withArgs {
		if ev.scope.Len() > 0 {
			e = e.Strs("scope", ev.scope.PackageNamesAsString())
		}

		for _, arg := range ev.arguments {
			res, err := serialize(arg.msg)
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
	sl.start(ra.data, true).Msg("start")
}

func (sl *sinkLogger) Done(ra *RunningAction) {
	ev := sl.start(ra.data, true)
	if ra.data.err != nil {
		t := errorType(ra.data.err)
		switch t {
		case errIsCancelled, errIsDependencyFailed:
			ev.Msg(string(t))
			return
		default:
			ev = ev.Stack().Err(ra.data.err)
		}
	}
	ev.Dur("took", ra.data.completed.Sub(ra.data.started)).Msg("done")
}

func (sl *sinkLogger) Instant(ev *EventData) {
	sl.start(*ev, true).Msg(ev.name)
}

func (sl *sinkLogger) AttachmentsUpdated(string, *resultData) { /* nothing to do */ }