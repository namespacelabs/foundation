// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package renderwait

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/kr/text"
	"github.com/morikuni/aec"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/text/timefmt"
	"namespacelabs.dev/foundation/schema/orchestration"
)

type consRenderer struct {
	ch        chan *orchestration.Event
	done      chan struct{}
	flushLog  io.Writer
	setSticky func(string)
}

type blockState struct {
	Category              string
	Title                 string
	Stage                 orchestration.Event_Stage
	AlreadyExisted        bool
	Start, Committed, End time.Time
	WaitStatus            []*orchestration.Event_WaitStatus
	WaitDetails           string
}

func (rwb consRenderer) Ch() chan *orchestration.Event { return rwb.ch }
func (rwb consRenderer) Wait(ctx context.Context) error {
	select {
	case <-rwb.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (rwb consRenderer) Loop(ctx context.Context) {
	defer close(rwb.done) // Signal parent we're done.

	resourceState := map[string]*blockState{} // Key: ResourceId
	ids := []string{}

	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-rwb.ch:
			if !ok {
				fmt.Fprint(rwb.flushLog, render(resourceState, ids, true))
				rwb.setSticky("")
				return
			}

			stage := ev.Stage
			// Backwards compatibility.
			if stage == orchestration.Event_UNKNOWN_STAGE {
				if ev.Ready == orchestration.Event_READY {
					stage = orchestration.Event_DONE
				} else {
					stage = orchestration.Event_WAITING
				}
			}

			var timestamp time.Time
			if ev.Timestamp != nil {
				timestamp = ev.Timestamp.AsTime()
			} else {
				timestamp = time.Now()
			}

			if _, has := resourceState[ev.ResourceId]; !has {
				ids = append(ids, ev.ResourceId)
				sort.Strings(ids)

				title := ev.Scope
				if title == "" {
					title = ev.ResourceId
				}

				resourceState[ev.ResourceId] = &blockState{
					Category: ev.Category,
					Title:    title,
					Stage:    stage,
					Start:    timestamp,
				}
			}

			resourceState[ev.ResourceId].AlreadyExisted = ev.AlreadyExisted

			if stage >= orchestration.Event_COMMITTED && resourceState[ev.ResourceId].Committed.IsZero() {
				resourceState[ev.ResourceId].Committed = timestamp
			}
			if stage >= orchestration.Event_DONE && resourceState[ev.ResourceId].End.IsZero() {
				resourceState[ev.ResourceId].End = timestamp
			}

			resourceState[ev.ResourceId].Stage = stage
			resourceState[ev.ResourceId].WaitStatus = ev.WaitStatus

			if stage != orchestration.Event_DONE && ev.WaitDetails != "" {
				resourceState[ev.ResourceId].WaitDetails = ev.WaitDetails
			}

			rwb.setSticky(render(resourceState, ids, false))
		}
	}
}

func render(m map[string]*blockState, ids []string, flush bool) string {
	var b bytes.Buffer
	if flush {
		fmt.Fprintln(&b)
	}

	cats := map[string]struct{}{}
	for _, bs := range m {
		cats[bs.Category] = struct{}{}
	}

	sortedCats := maps.Keys(cats)
	slices.Sort(sortedCats)

	perCat := map[string][]*blockState{}

	for _, id := range ids {
		x := m[id]
		perCat[x.Category] = append(perCat[x.Category], x)
	}

	for k, cat := range sortedCats {
		if k > 0 {
			fmt.Fprintln(&b)
		}

		blocks := perCat[cat]
		if len(blocks) == 0 {
			continue
		}

		fmt.Fprintf(&b, " %s:\n\n", cat)
		for _, blk := range blocks {
			var ready bool
			var took string
			if blk.AlreadyExisted && blk.Stage != orchestration.Event_DONE {
				took = box("Waiting for previous deployment ...", mergeWaitStatus(blk.WaitStatus))
			} else if blk.AlreadyExisted {
				ready = true
				took = "(no update required)"
			} else if blk.Stage == orchestration.Event_DONE {
				ready = true
				took = fmt.Sprintf("took %v (waited %v)", timefmt.Format(blk.End.Sub(blk.Committed)), timefmt.Format(blk.Committed.Sub(blk.Start)))
			} else {
				took = mergeWaitStatus(blk.WaitStatus)
				if took == "" {
					took = "Waiting..."
				}
			}

			fmt.Fprintf(&b, "  %s %s %s\n", icon(ready), blk.Title, aec.LightBlackF.Apply(took))
			if details := blk.WaitDetails; !ready && details != "" {
				fmt.Fprint(text.NewIndentWriter(&b, []byte("      ")), details)
			}
		}
	}

	if flush {
		fmt.Fprintln(&b)
	}
	return b.String()
}

func icon(ready bool) string {
	if ready {
		return "[✓]"
	}
	return "[ ]"
}

func mergeWaitStatus(status []*orchestration.Event_WaitStatus) string {
	var st []string
	for _, s := range status {
		st = append(st, s.Description)
	}
	return strings.Join(st, "; ")
}

func box(a, b string) string {
	if b == "" {
		return a
	}
	return fmt.Sprintf("%s (%s)", a, b)
}
