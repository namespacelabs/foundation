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
	Category       string
	Title          string
	Ready          bool
	AlreadyExisted bool
	Start, End     time.Time
	WaitStatus     []*orchestration.Event_WaitStatus
	WaitDetails    string
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
					Ready:    ev.Ready == orchestration.Event_READY,
					Start:    time.Now(),
				}
			}

			resourceState[ev.ResourceId].AlreadyExisted = ev.AlreadyExisted
			resourceState[ev.ResourceId].Ready = ev.Ready == orchestration.Event_READY
			resourceState[ev.ResourceId].WaitStatus = ev.WaitStatus
			if resourceState[ev.ResourceId].Ready {
				resourceState[ev.ResourceId].End = time.Now()
			} else {
				if ev.WaitDetails != "" {
					resourceState[ev.ResourceId].WaitDetails = ev.WaitDetails
				}
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
			if blk.AlreadyExisted && !blk.Ready {
				took = box("Waiting for previous deployment ...", mergeWaitStatus(blk.WaitStatus))
			} else if blk.AlreadyExisted {
				ready = true
				took = "(no update required)"
			} else if blk.Ready {
				ready = true
				took = fmt.Sprintf("took %v", timefmt.Format(blk.End.Sub(blk.Start)))
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
