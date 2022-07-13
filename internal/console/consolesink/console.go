// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package consolesink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/morikuni/aec"
	"github.com/muesli/reflow/truncate"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/text/timefmt"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	OutputActionID        = false
	DisplayWaitingActions = false
	DebugConsoleOutput    = false
	DebugOutputDecisions  = false
)

const (
	includeToolIDs         = false
	maxLogSourcesAccounted = 30 // Sources of the last [num] output messages.

	FPS = 60
)

var (
	// These assume a black background.
	ColorSticky   = aec.NewRGB8Bit(0x00, 0x2b, 0xac)
	ColorFade     = aec.LightBlackF
	ColorToolName = aec.Color8BitF(aec.NewRGB8Bit(0x30, 0x30, 0x30))
	ColorToolId   = aec.Color8BitF(aec.NewRGB8Bit(0x30, 0x30, 0x30))

	ColorsToolBar = []aec.RGB8Bit{
		// 7 items to provide for better distribution against ids.
		aec.NewRGB8Bit(0x56, 0x00, 0xac),
		aec.NewRGB8Bit(0x56, 0x00, 0xd7),
		aec.NewRGB8Bit(0x56, 0x2b, 0xd7),
		aec.NewRGB8Bit(0x56, 0x56, 0xd7),
		aec.NewRGB8Bit(0x56, 0x81, 0xd7),
		aec.NewRGB8Bit(0x56, 0xac, 0xd7),
		aec.NewRGB8Bit(0x56, 0xd7, 0xd7),
	}

	StickyRequired = map[string]bool{
		"webui":    true,
		"commands": true,
	}

	StickyPriorities = map[string]int{
		"stack":    1,
		"webui":    10,
		"commands": 20,
	}
)

var (
	usBar     = aec.LightCyanB.Apply(" ")
	stickyBar = aec.Color8BitB(ColorSticky).Apply(" ")
	toolBars  []string
)

func init() {
	for _, t := range ColorsToolBar {
		toolBars = append(toolBars, aec.Color8BitB(t).Apply(" ")+" ")
	}
}

type consoleOutput struct {
	id       common.IdAndHash
	actionID tasks.ActionID
	name     string
	cat      common.CatOutputType
	lines    [][]byte
}

type consoleEvent struct {
	output    consoleOutput
	setSticky struct {
		name     string
		contents []byte
	}

	attachmentUpdatedForID tasks.ActionID   // Set if we got an attachments updated message.
	ev                     tasks.EventData  // Set on Start() and Done().
	results                tasks.ResultData // Set on Done() or AttachmentsUpdated().
	progress               tasks.ActionProgress

	renderingMode string        // One of "rendering", or "input". In "input", rendering is disabled.
	onInput       chan struct{} // When the console enters the input mode, the console closes this channel.
}

type atom struct {
	key    string
	value  string
	result bool
}

type ConsoleSink struct {
	out           *os.File
	interactive   bool          // If set to false, only emit buffer output and logged actions.
	outbuf        *bytes.Buffer // A buffer is utilized when preparing output, to avoiding having multiple individual writes hit the console.
	previousLines [][]byte      // We keep a copy of the last rendered lines to avoid redrawing if the output doesn't change.

	waitDone chan struct{}
	ch       chan consoleEvent
	ticker   <-chan time.Time

	idling          bool
	buffer          []consoleOutput          // Pending regular log output lines.
	running         []*lineItem              // Computed EventData for waiting/running actions.
	root            *node                    // Root of the tree of displayable events.
	nodes           map[tasks.ActionID]*node // Map of actionID->tree node.
	startedCounting time.Time                // When did we start counting.
	waitForIdle     []chan struct{}

	maxLevel int // Only display actions at this level or below (all actions are still computed).

	idleLabel     string           // Label that is shown after `[-] idle` when no tasks are running.
	stickyContent []*stickyContent // Multi-line content that is always displayed above actions.

	logSources logSources // Sources of log output (think: action IDs)

	debugOut *json.Encoder
}

type stickyContent struct {
	name    string
	content [][]byte
}

type lineItem struct {
	data       tasks.EventData // The original event data.
	results    tasks.ResultData
	scope      []string             // List of packages this line item pertains to.
	serialized []atom               // Pre-rendered arguments.
	cached     bool                 // Whether this item represents a cache hit.
	progress   tasks.ActionProgress // This is not great, as we're using memory sharing here, but keeping it simple.
}

type node struct {
	item        *lineItem
	replacement *node // If this node is a `compute.wait`, replace it with the actual computation it is waiting on.
	hidden      bool  // Whether this node has been marked hidden (because e.g. there are multiple nodes to the same anchor).
	children    []tasks.ActionID
}

type logSources struct {
	mu      sync.Mutex
	sources []tasks.ActionID // Sources of log output (think: action IDs)
}

var _ tasks.ActionSink = &ConsoleSink{}

func NewSink(out *os.File, interactive bool, maxLevel int) *ConsoleSink {
	return &ConsoleSink{
		out:         out,
		interactive: interactive,
		outbuf:      bytes.NewBuffer(make([]byte, 4*1024)), // Start with 4k, enough to hold 20 lines of 100 bytes. bytes.Buffer will grow as needed.
		idling:      true,
		maxLevel:    maxLevel,
	}
}

func (c *ConsoleSink) Start() func() {
	c.waitDone = make(chan struct{})

	// We never close `ch`; the reason for that is that it would require a lot of coordination
	// across all possible goroutines that can spawn actions, produce logging, etc. This
	// design needs a bit of re-thinking as we do need central coordination for rendering, but
	// perhaps writing to a buffer would be sufficient.
	ch := make(chan consoleEvent)
	c.ch = ch

	interval := (1000 / FPS) * time.Millisecond
	if DebugConsoleOutput {
		interval = 300 * time.Millisecond
	}
	t := time.NewTicker(interval)
	c.ticker = t.C

	var out *os.File
	if DebugOutputDecisions {
		var err error
		out, err = os.CreateTemp("", "consoledebug")
		if err != nil {
			panic(err)
		}
		c.debugOut = json.NewEncoder(out)
		c.debugOut.SetIndent("", "  ")

		log.Println("Debugging console to", out.Name())
	}

	done := make(chan struct{})
	go c.run(done)

	return func() {
		close(done)
		<-c.waitDone
		t.Stop()
		if out != nil {
			out.Close()
			log.Println("Debugged console to", out.Name())
		}
	}
}

func (c *ConsoleSink) run(canceled chan struct{}) {
	defer close(c.waitDone)

	rendering := true
loop:
	for {
		select {
		case <-canceled:
			break loop

		case msg := <-c.ch:
			if msg.renderingMode != "" {
				if msg.renderingMode == "rendering" {
					rendering = true
				} else {
					c.waitForIdle = append(c.waitForIdle, msg.onInput)
				}
			}

			if msg.output.lines != nil {
				c.buffer = append(c.buffer, msg.output)
			}

			if msg.setSticky.name != "" {
				found := false
				for k, sc := range c.stickyContent {
					if sc.name == msg.setSticky.name {
						if len(msg.setSticky.contents) == 0 {
							c.stickyContent = append(c.stickyContent[:k], c.stickyContent[k+1:]...)
						} else {
							sc.content = bytes.Split(msg.setSticky.contents, []byte("\n"))
						}
						found = true
						break
					}
				}

				if !found && len(msg.setSticky.contents) > 0 {
					c.stickyContent = append(c.stickyContent, &stickyContent{
						name:    msg.setSticky.name,
						content: bytes.Split(msg.setSticky.contents, []byte("\n")),
					})

					slices.SortStableFunc(c.stickyContent, func(a, b *stickyContent) bool {
						var acost, bcost int

						if p, ok := StickyPriorities[a.name]; ok {
							acost = p
						} else {
							acost = math.MaxInt
						}

						if p, ok := StickyPriorities[b.name]; ok {
							bcost = p
						} else {
							bcost = math.MaxInt
						}

						return acost < bcost
					})
				}
			}

			if msg.attachmentUpdatedForID != "" {
				item := c.addOrGet(msg.attachmentUpdatedForID, false)
				if item != nil {
					item.results = msg.results
					if msg.progress != nil {
						item.progress = msg.progress
					}
					item.precompute()
					// recomputeTree is not required because parent/children relationships have not changed.
				}
			}

			if msg.ev.ActionID != "" {
				item := c.addOrGet(msg.ev.ActionID, true)
				item.data = msg.ev
				item.results = msg.results
				item.progress = msg.progress
				item.precompute()
				c.recomputeTree()
			}

		case t := <-c.ticker:
			if !rendering {
				continue
			}
			running, waiting, anchored := c.countStates()
			if running+waiting+anchored == 0 {
				c.redraw(t, true) // Entering input mode - flush the state.
				for _, waitingForIdle := range c.waitForIdle {
					// Unblock callers waiting for the input state.
					close(waitingForIdle)
				}
				rendering = len(c.waitForIdle) == 0
				c.waitForIdle = nil
			} else {
				c.redraw(t, false)
			}
		}
	}

	// Flush anything that is pending.
	c.redraw(time.Now(), true)
}

func (c *ConsoleSink) addOrGet(actionID tasks.ActionID, addIfMissing bool) *lineItem {
	index := -1
	for k, r := range c.running {
		if r.data.ActionID == actionID {
			index = k
		}
	}

	if index < 0 {
		if !addIfMissing {
			return nil
		}

		index = len(c.running)
		c.running = append(c.running, &lineItem{})
	}

	return c.running[index]
}

func (li *lineItem) precompute() {
	data := li.data

	var serialized []atom

	if data.AnchorID != "" {
		serialized = append(serialized, atom{key: "anchor", value: data.AnchorID.String()})
	}

	li.scope = data.Scope.PackageNamesAsString()

	for _, arg := range data.Arguments {
		var value string

		switch arg.Name {
		case "cached":
			if b, ok := arg.Msg.(bool); ok && b {
				li.cached = true
			}

		default:
			if b, err := common.SerializeToBytes(arg.Msg); err == nil {
				value = string(b)
			} else {
				value = fmt.Sprintf("failed to serialize to json: %v", err)
			}
		}

		if value != "" {
			serialized = append(serialized, atom{key: arg.Name, value: value})
		}
	}

	for _, r := range li.results.Items {
		var value string

		if b, err := common.SerializeToBytes(r.Msg); err == nil {
			value = string(b)
		} else {
			value = fmt.Sprintf("failed to serialize to json: %v", err)
		}

		if value != "" {
			serialized = append(serialized, atom{key: r.Name, value: value, result: true})
		}
	}

	li.serialized = serialized
}

func (c *ConsoleSink) recomputeTree() {
	nodes := map[tasks.ActionID]*node{}
	root := &node{}
	anchors := map[tasks.ActionID]bool{}

	var runningCount int
	for _, item := range c.running {
		nodes[item.data.ActionID] = &node{item: item}
		if !item.data.Indefinite {
			runningCount++
		}
	}

	for _, item := range c.running {
		r := item.data
		parent := parentOf(root, nodes, r.ParentID)
		parent.children = append(parent.children, r.ActionID)

		if r.AnchorID != "" && nodes[r.AnchorID] != nil {
			// We used to replace "waiting" nodes with the lines they're waiting on.
			// But that turned out to be confusing when there are multiple waiters,
			// because it seems like we're doing the same work N times. So now we
			// only do it once.
			if !anchors[r.AnchorID] {
				nodes[r.ActionID].replacement = nodes[r.AnchorID]
				anchors[r.AnchorID] = true
			} else {
				nodes[r.ActionID].hidden = true
			}
		}
	}

	// If a line item has at least one anchor, unattached from it's original root.
	for anchorID := range anchors {
		anchorParent := parentOf(root, nodes, nodes[anchorID].item.data.ParentID)
		anchorParent.children = without(anchorParent.children, anchorID)
	}

	c.root = root
	c.nodes = nodes
	sortNodes(nodes, root)

	if runningCount > 0 {
		if c.idling {
			c.idling = false
			c.startedCounting = time.Now()
		}
	} else {
		c.idling = true
	}
}

func without(strs []tasks.ActionID, str tasks.ActionID) []tasks.ActionID {
	var newStrs []tasks.ActionID
	for _, s := range strs {
		if s != str {
			newStrs = append(newStrs, s)
		}
	}
	return newStrs
}

func sortNodes(nodes map[tasks.ActionID]*node, n *node) {
	sort.Slice(n.children, func(i, j int) bool {
		// If an action is anchored, use the anchor's start time for sorting purposes.
		a := follow(nodes[n.children[i]])
		b := follow(nodes[n.children[j]])
		return a.item.data.Started.Before(b.item.data.Started)
	})

	for _, id := range n.children {
		sortNodes(nodes, nodes[id])
	}
}

func follow(n *node) *node {
	if n.replacement != nil {
		return n.replacement
	}
	return n
}

func parentOf(root *node, tree map[tasks.ActionID]*node, id tasks.ActionID) *node {
	if id == "" {
		return root
	} else {
		if p, ok := tree[id]; !ok {
			// Referenced parent doesn't exist, attach to root.
			return root
		} else {
			return p
		}
	}
}

type debugData struct {
	Width       uint
	Height      uint
	Flush       bool
	Previous    uint
	BufferCount int
	Running     []debugRunning
}

type debugRunning struct {
	ID        tasks.ActionID
	Name      string
	Created   time.Time
	State     string
	Completed *time.Time
}

func (c *ConsoleSink) redraw(t time.Time, flush bool) {
	if !c.interactive {
		c.drawFrame(c.out, c.outbuf, t, 0, 0, flush)
		return
	}

	var width uint
	var height uint
	if w, err := termios.TermSize(c.out.Fd()); err == nil {
		width = uint(w.Width)
		height = uint(w.Height)
	}

	previousLines := c.previousLines

	cursorWasReset := false
	resetCursorOnce := func() {
		if !cursorWasReset && c.interactive {
			// Hide the cursor while re-rendering.
			fmt.Fprint(c.out, aec.Hide)
			if !DebugConsoleOutput {
				if x := uint(len(previousLines)); x > 0 {
					fmt.Fprint(c.out, aec.Up(x))
				}
			}
			cursorWasReset = true
		}
	}

	defer func() {
		if cursorWasReset {
			fmt.Fprint(c.out, aec.Show)
		}
	}()

	rawOut := checkDirtyWriter{out: c.out, onFirstWrite: func() {
		resetCursorOnce()
		// If anything is trying to write directly, clear the screen first.
		fmt.Fprint(c.out, aec.EraseDisplay(aec.EraseModes.Tail))
	}}

	c.drawFrame(&rawOut, c.outbuf, t, width, height, flush)

	newFrame := make([]byte, len(c.outbuf.Bytes()))
	copy(newFrame, c.outbuf.Bytes())
	c.outbuf.Reset()

	newLines := bytes.Split(bytes.TrimSpace(newFrame), []byte("\n"))
	c.previousLines = newLines

	// If there was no console buffer output, check if perhaps we're emitting exactly the same.
	// If that's the case, then skip re-rendering, to avoid moving the cursor around.
	if !rawOut.dirty && linesEqual(previousLines, newLines) {
		return
	}

	// Only reset the cursor if there's something to render.
	resetCursorOnce()

	for k, line := range newLines {
		if !rawOut.dirty && k < len(previousLines) && bytes.Equal(line, previousLines[k]) {
			// We could look for a common prefix here, and do a narrower clear. But
			// ANSI codes make this a bit complicated, as we can't easily cut a line
			// in the middle of an ansi sequence. So for now, we repaint the whole
			// line as needed.
			fmt.Fprint(c.out, aec.Down(1))
			continue
		}

		fmt.Fprint(c.out, aec.EraseLine(aec.EraseModes.Tail))
		_, _ = c.out.Write(line)
		fmt.Fprint(c.out, "\n\r")
	}

	// Clear any previously rendering additional lines.
	fmt.Fprint(c.out, aec.EraseDisplay(aec.EraseModes.Tail))
}

func linesEqual(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, aline := range a {
		if !bytes.Equal(aline, b[k]) {
			return false
		}
	}
	return true
}

type checkDirtyWriter struct {
	out          io.Writer
	dirty        bool
	onFirstWrite func()
}

func (c *checkDirtyWriter) Write(p []byte) (int, error) {
	if !c.dirty {
		c.dirty = true
		if c.onFirstWrite != nil {
			c.onFirstWrite()
		}
	}
	return c.out.Write(p)
}

func (c *ConsoleSink) countStates() (running, waiting, anchored int) {
	running = 0
	waiting = 0
	anchored = 0
	for _, r := range c.running {
		if r.data.State == tasks.ActionRunning {
			if !r.data.Indefinite {
				if r.data.AnchorID != "" {
					anchored++
				} else {
					running++
				}
			}
		} else if r.data.State == tasks.ActionWaiting {
			waiting++
		}
	}
	return
}

// drawFrame renders a single frame and returns `false` if further rendering should be stopped.
func (c *ConsoleSink) drawFrame(raw, out io.Writer, t time.Time, width, height uint, flush bool) {
	running, waiting, anchored := c.countStates()
	var completed, completedAnchors int
	var printableCompleted []lineItem
	for _, r := range c.running {
		if r.data.State != tasks.ActionWaiting && r.data.State != tasks.ActionRunning {
			hasError := (r.data.Err != nil && tasks.ErrorType(r.data.Err) == tasks.ErrTypeIsRegular)
			shouldLog := tasks.LogActions && (DisplayWaitingActions || r.data.AnchorID == "")

			if (shouldLog || hasError) && r.data.Level <= c.maxLevel {
				printableCompleted = append(printableCompleted, *r)
			}

			completed++
			if r.data.AnchorID != "" {
				completedAnchors++
			}
		}
	}

	if tasks.LogActions && len(printableCompleted) > 0 {
		sort.Slice(printableCompleted, func(i, j int) bool {
			return printableCompleted[i].data.Completed.Before(printableCompleted[j].data.Completed)
		})

		for _, r := range printableCompleted {
			fmt.Fprint(raw, aec.EraseLine(aec.EraseModes.Tail))
			renderCompletedAction(raw, colors.WithColors, r)
		}
	}

	// Drain any pending logging message.
	var hdrBuf bytes.Buffer
	for _, block := range c.buffer {
		if block.name != "" && block.name != common.KnownStdout && block.name != common.KnownStderr {
			if block.cat == common.CatOutputUs {
				fmt.Fprint(&hdrBuf, usBar)
			} else {
				colorIndex := block.id.Digest % uint64(len(toolBars))
				if includeToolIDs {
					fmt.Fprint(&hdrBuf, toolBars[colorIndex], ColorToolId.Apply(block.id.ID)+" "+ColorToolName.Apply(block.name))
				} else {
					fmt.Fprint(&hdrBuf, toolBars[colorIndex], ColorToolName.Apply(block.name))
				}
			}
			for _, line := range block.lines {
				fmt.Fprintf(raw, "%s%s %s\n", aec.EraseLine(aec.EraseModes.Tail), hdrBuf.Bytes(), line)
				c.recordLogSource(block.actionID)
			}
			hdrBuf.Reset()
		} else {
			for _, line := range block.lines {
				fmt.Fprintf(raw, "%s%s\n", aec.EraseLine(aec.EraseModes.Tail), line)
				c.recordLogSource(block.actionID)
			}
		}
	}

	bufferCount := len(c.buffer)
	c.buffer = nil

	if c.debugOut != nil {
		var running []debugRunning

		for _, r := range c.running {
			var completed *time.Time
			if r.data.State == tasks.ActionDone {
				completed = &r.data.Completed
			}

			running = append(running, debugRunning{
				ID:        r.data.ActionID,
				Name:      r.data.Name,
				Created:   r.data.Created,
				State:     string(r.data.State),
				Completed: completed,
			})
		}

		_ = c.debugOut.Encode(debugData{
			Width:       width,
			Height:      height,
			Flush:       flush,
			Running:     running,
			BufferCount: bufferCount,
		})
	}

	// If at least one item has completed, re-compute the display tree. This is expensive
	// but kept simple for now. Will optimize later.
	if completed > 0 {
		var newRunning []*lineItem
		for _, r := range c.running {
			if r.data.State.IsRunning() {
				newRunning = append(newRunning, r)
			}
		}
		c.running = newRunning
		c.recomputeTree()
	}

	if !c.interactive {
		return
	}

	maxDepth, actionBlockLines, renderActions := c.calculateActionHeight(height)

	if len(c.stickyContent) > 0 {
		availableHeight := int(height-actionBlockLines) - 3 // Always leave at least 3 lines for logs.
		renderBlocks, surpressed := c.fitStickies(availableHeight)

		hdr := fmt.Sprintf("%s ", stickyBar)
		if availableHeight > 2 {
			c.writeLineWithMaxW(out, width, hdr, "")
		}
		for k, block := range renderBlocks {
			if k > 0 && len(block.content) > 0 {
				c.writeLineWithMaxW(out, width, hdr, "")
			}
			for _, line := range block.content {
				c.writeLineWithMaxW(out, width, fmt.Sprintf("%s%s", hdr, line), "")
			}
		}
		if len(surpressed) > 0 {
			c.writeLineWithMaxW(out, width, hdr, fmt.Sprintf("[... hiding %s]", strings.Join(surpressed, ", ")))

		}
		if availableHeight > 2 {
			c.writeLineWithMaxW(out, width, hdr, "")
		}
	}

	if (waiting + running + anchored) == 0 {
		if len(c.waitForIdle) == 0 && c.idleLabel != "" {
			// Show the idle banner.
			c.writeLineWithMaxW(out, width, fmt.Sprintf("[-] idle, %s.", c.idleLabel), "")
		}
		return
	}

	if flush {
		return
	}

	shortenLabel := ""
	if renderActions {
		// Recurse through the line item tree.
		c.renderLineRec(out, width, c.root, t, " => ", 0, maxDepth)
	} else {
		shortenLabel = "[...] "
	}

	report := ""
	report += fmt.Sprintf("[+] %s%s", shortenLabel, timefmt.Seconds(t.Sub(c.startedCounting)))
	report += fmt.Sprintf(" %s %s running", num(aec.GreenF, running), plural(running, "action", "actions"))
	if waiting > 0 {
		report += fmt.Sprintf(", %s waiting", num(aec.CyanF, waiting))
	}

	c.writeLineWithMaxW(out, width, report+".", "")
}

func (c *ConsoleSink) calculateActionHeight(height uint) (uint, uint, bool) {
	if c.root == nil {
		return 0, 0, false
	}

	// The height of the fixed action block.
	const reportLines = 2

	// The idea here is that we traverse the tree to figure out how many drawn lines would
	// have been emitted. And if we see too many, we try to reduce the tree depth, until
	// the number of lines is acceptable.
	maxDepth, lineCount := c.maxRenderDepth(c.root, 0, 16)

	if height <= reportLines {
		return 0, 0, false
	}

	maxHeight := (height - reportLines) / 2 // Only use up to half-height of actions.

	for lineCount > maxHeight {
		maxDepth--
		if maxDepth < 2 { // If we would go below 2, then we lose so much information may as well only render the report.
			return 0, 0, false
		}
		_, lineCount = c.maxRenderDepth(c.root, 0, maxDepth)
	}

	return lineCount, maxDepth, true
}

func (c *ConsoleSink) fitStickies(availableHeight int) ([]*stickyContent, []string) {
	if measureBlocksHeight(c.stickyContent) <= availableHeight {
		return c.stickyContent, nil
	}

	var surpressed []string
	var actual []*stickyContent

	for _, block := range c.stickyContent {
		if StickyRequired[block.name] {
			actual = append(actual, block)
		} else {
			surpressed = append(surpressed, block.name)
		}
	}

	if measureBlocksHeight(actual) <= availableHeight {
		return actual, surpressed
	}

	names := make([]string, len(c.stickyContent))
	for k, block := range c.stickyContent {
		names[k] = block.name
	}
	return nil, names
}

func measureBlocksHeight(blocks []*stickyContent) int {
	requiredHeight := 2 // Margin
	for k, block := range blocks {
		if k > 0 {
			requiredHeight++
		}
		requiredHeight += len(block.content)
	}
	return requiredHeight
}

func (c *ConsoleSink) recordLogSource(id tasks.ActionID) {
	c.logSources.mu.Lock()
	defer c.logSources.mu.Unlock()
	if len(c.logSources.sources) < maxLogSourcesAccounted {
		c.logSources.sources = append(c.logSources.sources, id)
		return
	}

	c.logSources.sources = append(c.logSources.sources[1:], id)
}

func (c *ConsoleSink) RecentInputSourcesContain(actionId tasks.ActionID) bool {
	c.logSources.mu.Lock()
	defer c.logSources.mu.Unlock()
	for _, logSource := range c.logSources.sources {
		if logSource == actionId {
			return true
		}
	}
	return false
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func num(c aec.ANSI, d int) string {
	return c.Apply(fmt.Sprintf("%d", d))
}

func (c *ConsoleSink) maxRenderDepth(n *node, currDepth, maxDepth uint) (uint, uint) {
	if currDepth >= maxDepth {
		return 0, 0
	}

	depth := uint(0)
	drawn := uint(0)
	for _, id := range n.children {
		child := follow(c.nodes[id])
		if child.hidden {
			// If hidden, don't even go through it's children.
			continue
		}

		subDepth, subDrawn := c.maxRenderDepth(child, currDepth+1, maxDepth)
		drawn += subDrawn
		if !skipRendering(child.item.data, c.maxLevel) {
			drawn++
		}

		subDepth++
		if subDepth > depth {
			depth = subDepth
		}
	}

	return depth, drawn
}

func skipRendering(data tasks.EventData, maxLevel int) bool {
	skip := data.Level > maxLevel
	skip = skip || data.Indefinite
	skip = skip || (!DisplayWaitingActions && data.State == tasks.ActionWaiting)
	return skip
}

func (c *ConsoleSink) renderLineRec(out io.Writer, width uint, n *node, t time.Time, inputPrefix string, currDepth, maxDepth uint) {
	if currDepth >= maxDepth {
		return
	}

	var lineb bytes.Buffer
	for _, id := range n.children {
		child := follow(c.nodes[id])

		if child.hidden {
			// If hidden, don't even go through it's children.
			continue
		}

		data := child.item.data

		prefix := inputPrefix
		if !skipRendering(data, c.maxLevel) {
			// Although this is not very efficient as we're thrashing strings, we need to make sure
			// we don't print more than one line, as that would disrupt the line acount we keep track
			// of to make for a smooth update in place.
			// XXX precompute these lines as they don't change if the arguments don't change.
			lineb.Reset()

			if OutputActionID {
				fmt.Fprint(&lineb, aec.LightBlackF.Apply(" ["+data.ActionID.String()[:8]+"]"))
			}

			fmt.Fprint(&lineb, prefix)

			renderLine(&lineb, colors.WithColors, *child.item)

			suffix := ""
			if data.State == tasks.ActionRunning {
				d := t.Sub(data.Started)
				suffix = " (" + timefmt.Seconds(d) + ") "
			} else if data.State == tasks.ActionWaiting {
				suffix = " (waiting) "
			}

			c.writeLineWithMaxW(out, width, lineb.String(), suffix)
			prefix += "=> "
		}

		c.renderLineRec(out, width, child, t, prefix, currDepth+1, maxDepth)
	}
}

func (c *ConsoleSink) writeLineWithMaxW(w io.Writer, width uint, line string, suffix string) {
	fmt.Fprintln(w, truncate.StringWithTail(line+suffix, width, " [...]"+suffix))
}

func (c *ConsoleSink) Waiting(ra *tasks.RunningAction) {
	c.ch <- consoleEvent{ev: ra.Data, progress: ra.Progress}
}

func (c *ConsoleSink) Started(ra *tasks.RunningAction) {
	c.ch <- consoleEvent{ev: ra.Data, progress: ra.Progress}
}

func (c *ConsoleSink) Done(ra *tasks.RunningAction) {
	// XXX lock ResultData
	c.ch <- consoleEvent{ev: ra.Data, results: ra.Attachments().ResultData}
}

func (c *ConsoleSink) Instant(ev *tasks.EventData) {
	c.ch <- consoleEvent{ev: *ev}
}

func (c *ConsoleSink) AttachmentsUpdated(actionID tasks.ActionID, data *tasks.ResultData) {
	if data != nil {
		c.ch <- consoleEvent{attachmentUpdatedForID: actionID, results: *data, progress: data.Progress}
	}
}

func (c *ConsoleSink) WriteLines(id common.IdAndHash, name string, cat common.CatOutputType, actionID tasks.ActionID, _ time.Time, lines [][]byte) {
	c.ch <- consoleEvent{output: consoleOutput{id: id, name: name, cat: cat, lines: lines, actionID: actionID}}
}

func (c *ConsoleSink) AllocateConsoleId() uint64 {
	return uint64(rand.Int63())
}

func (c *ConsoleSink) SetIdleLabel(label string) {
	// XXX locking
	c.idleLabel = label
}

func (c *ConsoleSink) SetStickyContent(name string, content []byte) {
	var ev consoleEvent
	ev.setSticky.name = name
	ev.setSticky.contents = content
	c.ch <- ev
}

// Stops rendering actions. But only does so when an idle state is entered, and
// blocks until that point.
func (c *ConsoleSink) EnterInputMode(ctx context.Context, prompt ...string) func() {
	inputCh := make(chan struct{}) // The console closes this channel when it enters input mode.
	c.ch <- consoleEvent{renderingMode: "input", onInput: inputCh}

	reenableRendering := func() {
		c.ch <- consoleEvent{renderingMode: "rendering"}
	}

	select {
	case <-inputCh:
		if len(prompt) > 0 {
			fmt.Fprint(os.Stdout, strings.Join(prompt, " "))
			os.Stdout.Sync()
		}
		return reenableRendering
	case <-ctx.Done():
		// Canceled while waiting, so lets turn rendering back on.
		reenableRendering()
		return func() {}
	}
}
