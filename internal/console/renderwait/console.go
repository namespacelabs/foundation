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
	"time"

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
	Status         string
}

func (rwb consRenderer) Ch() chan ops.Event { return rwb.ch }
func (rwb consRenderer) Wait()              { <-rwb.done }

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
				return
			}

			if ev.AllDone {
				rwb.flushLog.Write(render(m, ids, true))
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
			m[ev.ResourceID].Status = ev.Status
			if m[ev.ResourceID].Ready {
				m[ev.ResourceID].End = time.Now()
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

		c := perCat[cat]
		if len(c) == 0 {
			continue
		}

		fmt.Fprintf(&b, " %s:\n\n", cat)
		for _, blk := range c {
			var icon, took string
			if blk.AlreadyExisted && !blk.Ready {
				icon = "[ ]"
				took = "(no update required, waiting for old deployment)"
				if blk.Status != "" {
					took = fmt.Sprintf("(no update required, waiting for old deployment, last deployment status: %s)", blk.Status)
				}
			} else if blk.AlreadyExisted {
				icon = "[✓]"
				took = "(no update required)"
			} else if blk.Ready {
				icon = "[✓]"
				took = fmt.Sprintf("took %v", timefmt.Format(blk.End.Sub(blk.Start)))
			} else {
				icon = "[ ]"
				took = "waiting ..."
				if blk.Status != "" {
					took = fmt.Sprintf("waiting ... (last deployment status: %s)", blk.Status)
				}
			}
			fmt.Fprintf(&b, "  %s %s %s\n", icon, blk.Scope, aec.LightBlackF.Apply(took))
		}
	}

	if flush {
		fmt.Fprintln(&b)
	}
	return b.Bytes()
}
