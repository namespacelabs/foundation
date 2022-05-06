// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/text/timefmt"
	"namespacelabs.dev/foundation/internal/uniquestrings"
)

type consRenderer struct {
	ch        chan ops.Event
	done      chan struct{}
	flushLog  io.Writer
	setSticky func([]byte)
}

type blockState struct {
	Category       string
	Scope          string
	Ready          bool
	AlreadyExisted bool
	Start, End     time.Time
	WaitStatus     []ops.WaitStatus
	WaitDetails    string
}

func (rwb consRenderer) Ch() chan ops.Event { return rwb.ch }
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

	m := map[string]*blockState{}
	ids := []string{}

	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-rwb.ch:
			if !ok {
				_, _ = rwb.flushLog.Write(render(m, ids, true))
				rwb.setSticky(nil)
				return
			}

			if _, has := m[ev.ResourceID]; !has {
				ids = append(ids, ev.ResourceID)
				sort.Strings(ids)

				m[ev.ResourceID] = &blockState{
					Category: ev.Category,
					Scope:    ev.Scope.String(),
					Ready:    ev.Ready == ops.Ready,
					Start:    time.Now(),
				}
			}

			m[ev.ResourceID].AlreadyExisted = ev.AlreadyExisted
			m[ev.ResourceID].Ready = ev.Ready == ops.Ready
			m[ev.ResourceID].WaitStatus = ev.WaitStatus
			if m[ev.ResourceID].Ready {
				m[ev.ResourceID].End = time.Now()
			} else {
				if ev.WaitDetails != "" {
					m[ev.ResourceID].WaitDetails = ev.WaitDetails
				}
			}

			rwb.setSticky(render(m, ids, false))
		}
	}
}

func render(m map[string]*blockState, ids []string, flush bool) []byte {
	var b bytes.Buffer
	if flush {
		fmt.Fprintln(&b)
	}

	var cats uniquestrings.List
	for _, bs := range m {
		cats.Add(bs.Category)
	}

	sortedCats := cats.Strings()
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
				took = box("waiting for previous deployment ...", mergeWaitStatus(blk.WaitStatus))
			} else if blk.AlreadyExisted {
				ready = true
				took = "(no update required)"
			} else if blk.Ready {
				ready = true
				took = fmt.Sprintf("took %v", timefmt.Format(blk.End.Sub(blk.Start)))
			} else {
				took = mergeWaitStatus(blk.WaitStatus)
				if took == "" {
					took = "waiting ..."
				}
			}

			fmt.Fprintf(&b, "  %s %s %s\n", icon(ready), blk.Scope, aec.LightBlackF.Apply(took))
			if details := blk.WaitDetails; !ready && details != "" {
				fmt.Fprint(text.NewIndentWriter(&b, []byte("      ")), details)
			}
		}
	}

	if flush {
		fmt.Fprintln(&b)
	}
	return b.Bytes()
}

func icon(ready bool) string {
	if ready {
		return "[✓]"
	}
	return "[ ]"
}

func mergeWaitStatus(status []ops.WaitStatus) string {
	var st []string
	for _, s := range status {
		st = append(st, s.WaitStatus())
	}
	return strings.Join(st, "; ")
}

func box(a, b string) string {
	if b == "" {
		return a
	}
	return fmt.Sprintf("%s (%s)", a, b)
}
