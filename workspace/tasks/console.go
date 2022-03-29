// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/morikuni/aec"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/logoutput"
	"namespacelabs.dev/foundation/internal/text/timefmt"
)

var (
	LogActions            = false
	OutputActionID        = false
	DisplayWaitingActions = false
	DebugConsoleOutput    = false
	DebugOutputDecisions  = false
)

const (
	KnownStdout = "fn.console.stdout"
	KnownStderr = "fn.console.stderr"

	CatOutputTool     CatOutputType = "fn.output.tool"
	CatOutputUs       CatOutputType = "fn.output.foundation"
	CatOutputWarnings CatOutputType = "fn.output.warnings"
	CatOutputErrors   CatOutputType = "fn.output.errors"

	includeToolIDs = false
)

var (
	// These assume a black background.
	ColorSticky   = aec.NewRGB8Bit(0x00, 0x2b, 0xac)
	ColorFade     = aec.LightBlackF
	ColorToolName = aec.Color8BitF(aec.NewRGB8Bit(0x30, 0x30, 0x30))
	ColorToolId   = aec.Color8BitF(aec.NewRGB8Bit(0x30, 0x30, 0x30))
	ColorPackage  = aec.Italic

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

type CatOutputType string

type consoleOutput struct {
	id    IdAndHash
	name  string
	cat   CatOutputType
	lines [][]byte
}

type IdAndHash struct {
	id     string
	digest uint64
}

func IdAndHashFrom(id string) IdAndHash {
	return IdAndHash{id: id, digest: xxhash.Sum64String(id)}
}

type consoleEvent struct {
	output    consoleOutput
	setSticky struct {
		name     string
		contents []byte
	}

	attachmentUpdatedForID string     // Set if we got an attachments updated message.
	ev                     EventData  // Set on Start() and Done().
	results                resultData // Set on Done() or AttachmentsUpdated().
	progress               ActionProgress

	renderingMode string        // One of "rendering", or "input". In "input", rendering is disabled.
	onInput       chan struct{} // When the console enters the input mode, the console closes this channel.
}

type atom struct {
	key    string
	value  string
	result bool
}

type ConsoleSink struct {
	out       *os.File
	outbuf    *bytes.Buffer // A buffer is utilized when preparing output, to avoiding having multiple individual writes hit the console.
	lastFrame []byte        // We keep a copy of the last rendered frame to avoid redrawing if the output doesn't change.

	waitDone chan struct{}
	ch       chan consoleEvent
	ticker   <-chan time.Time

	rendering   bool
	buffer      []consoleOutput  // Pending regular log output lines.
	running     []*lineItem      // Computed EventData for waiting/running actions.
	root        *node            // Root of the tree of displayable events.
	nodes       map[string]*node // Map of actionID->tree node.
	previous    uint             // How many lines we displayed previously.
	started     time.Time        // When did we start counting.
	waitForIdle []func() bool

	maxLevel int // Only display actions at this level or below (all actions are still computed).

	idleLabel     string           // Label that is shown after `[-] idle` when no tasks are running.
	stickyContent []*stickyContent // Multi-line content that is always displayed above actions.

	debugOut *json.Encoder
}

type stickyContent struct {
	name    string
	content [][]byte
}

type lineItem struct {
	data       EventData // The original event data.
	results    resultData
	scope      []string       // List of packages this line item pertains to.
	serialized []atom         // Pre-rendered arguments.
	cached     bool           // Whether this item represents a cache hit.
	progress   ActionProgress // This is not great, as we're using memory sharing here, but keeping it simple.
}

type node struct {
	item        *lineItem
	replacement *node // If this node is a `compute.wait`, replace it with the actual computation it is waiting on.
	hidden      bool  // Whether this node has been marked hidden (because e.g. there are multiple nodes to the same anchor).
	children    []string
}

var _ ActionSink = &ConsoleSink{}

func NewConsoleSink(out *os.File, maxLevel int) *ConsoleSink {
	return &ConsoleSink{
		out:       out,
		outbuf:    bytes.NewBuffer(make([]byte, 4*1024)), // Start with 4k, enough to hold 20 lines of 100 bytes. bytes.Buffer will grow as needed.
		rendering: true,
		maxLevel:  maxLevel,
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

	interval := 100 * time.Millisecond
	if DebugConsoleOutput {
		interval = 300 * time.Millisecond
	}
	t := time.NewTicker(interval)
	c.ticker = t.C

	c.idleLabel = "nothing to do"

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

loop:
	for {
		select {
		case <-canceled:
			break loop

		case msg := <-c.ch:
			if msg.renderingMode != "" {
				if msg.renderingMode == "rendering" {
					c.rendering = true
				} else {
					ch := msg.onInput
					c.waitForIdle = append(c.waitForIdle, func() bool {
						c.rendering = false
						if ch != nil {
							close(ch)
						}
						return true
					})
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

			if msg.ev.actionID != "" {
				item := c.addOrGet(msg.ev.actionID, true)
				item.data = msg.ev
				item.results = msg.results
				item.progress = msg.progress
				item.precompute()
				c.recomputeTree()
			}

		case t := <-c.ticker:
			c.redraw(t, false)
		}
	}

	// Flush anything that is pending.
	c.redraw(time.Now(), true)
}

func (c *ConsoleSink) addOrGet(actionID string, addIfMissing bool) *lineItem {
	index := -1
	for k, r := range c.running {
		if r.data.actionID == actionID {
			index = k
		}
	}

	if index < 0 {
		if !addIfMissing {
			return nil
		}

		index = len(c.running)
		if index == 0 {
			c.started = time.Now()
		}
		c.running = append(c.running, &lineItem{})
	}

	return c.running[index]
}

func (li *lineItem) precompute() {
	data := li.data

	var serialized []atom

	if data.anchorID != "" {
		serialized = append(serialized, atom{key: "anchor", value: data.anchorID})
	}

	li.scope = data.scope.PackageNamesAsString()

	for _, arg := range data.arguments {
		var value string

		switch arg.Name {
		case "cached":
			if b, ok := arg.msg.(bool); ok && b {
				li.cached = true
			}

		default:
			if s, err := serialize(arg.msg); err == nil {
				if b, err := serializeToBytes(s); err == nil {
					value = string(b)
				} else {
					value = fmt.Sprintf("failed to serialize to json: %v", err)
				}
			} else {
				value = fmt.Sprintf("failed to serialize: %v", err)
			}
		}

		if value != "" {
			serialized = append(serialized, atom{key: arg.Name, value: value})
		}
	}

	for _, r := range li.results.items {
		var value string

		if s, err := serialize(r.msg); err == nil {
			if b, err := serializeToBytes(s); err == nil {
				value = string(b)
			} else {
				value = fmt.Sprintf("failed to serialize to json: %v", err)
			}
		} else {
			value = fmt.Sprintf("failed to serialize: %v", err)
		}

		if value != "" {
			serialized = append(serialized, atom{key: r.Name, value: value, result: true})
		}
	}

	li.serialized = serialized
}

func (c *ConsoleSink) recomputeTree() {
	nodes := map[string]*node{}
	root := &node{}
	anchors := map[string]bool{}

	for _, item := range c.running {
		nodes[item.data.actionID] = &node{item: item}
	}

	for _, item := range c.running {
		r := item.data
		parent := parentOf(root, nodes, r.parentID)
		parent.children = append(parent.children, r.actionID)

		if r.anchorID != "" && nodes[r.anchorID] != nil {
			// We used to replace "waiting" nodes with the lines they're waiting on.
			// But that turned out to be confusing when there are multiple waiters,
			// because it seems like we're doing the same work N times. So now we
			// only do it once.
			if !anchors[r.anchorID] {
				nodes[r.actionID].replacement = nodes[r.anchorID]
				anchors[r.anchorID] = true
			} else {
				nodes[r.actionID].hidden = true
			}
		}
	}

	// If a line item has at least one anchor, unattached from it's original root.
	for anchorID := range anchors {
		anchorParent := parentOf(root, nodes, nodes[anchorID].item.data.parentID)
		anchorParent.children = without(anchorParent.children, anchorID)
	}

	c.root = root
	c.nodes = nodes
	sortNodes(nodes, root)
}

func without(strs []string, str string) []string {
	var newStrs []string
	for _, s := range strs {
		if s != str {
			newStrs = append(newStrs, s)
		}
	}
	return newStrs
}

func sortNodes(nodes map[string]*node, n *node) {
	sort.Slice(n.children, func(i, j int) bool {
		// If an action is anchored, use the anchor's start time for sorting purposes.
		a := follow(nodes[n.children[i]])
		b := follow(nodes[n.children[j]])
		return a.item.data.started.Before(b.item.data.started)
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

func parentOf(root *node, tree map[string]*node, id string) *node {
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

func renderLine(w io.Writer, li *lineItem) {
	data := li.data

	base := aec.EmptyBuilder.ANSI

	if data.state.IsDone() {
		// XXX using UTC() here to be consistent with zerolog.ConsoleWriter.
		t := data.completed.UTC().Format(logoutput.StampMilliTZ)
		fmt.Fprint(w, base.With(aec.LightBlackF).Apply(t), " ")

		if OutputActionID {
			fmt.Fprint(w, aec.LightBlackF.Apply("["+data.actionID[:8]+"] "))
		}
	}

	if data.category != "" {
		fmt.Fprint(w, base.With(aec.LightBlueF).Apply("("+data.category+") "))
	}

	name := data.humanReadable
	if name == "" {
		name = data.name
	}

	if li.cached {
		fmt.Fprint(w, base.With(aec.LightBlackF).Apply(name))
	} else {
		fmt.Fprint(w, name)
	}

	if progress := li.progress; progress != nil && data.state == actionRunning {
		if p := progress.FormatProgress(); p != "" {
			fmt.Fprint(w, " ", base.With(aec.LightBlackF).Apply(p))
		}
	}

	if data.humanReadable == "" && len(li.scope) > 0 {
		fmt.Fprint(w, " "+ColorPackage.String()+"[")
		scope := li.scope
		var origlen int
		if len(scope) > 3 {
			origlen = len(scope)
			scope = scope[:3]
		}

		for k, pkg := range scope {
			if k > 0 {
				fmt.Fprint(w, " ")
			}
			fmt.Fprint(w, pkg)
		}

		if origlen > 0 {
			fmt.Fprintf(w, " and %d more", origlen-len(scope))
		}

		fmt.Fprint(w, "]"+aec.Reset)
	}

	for _, kv := range li.serialized {
		color := aec.CyanF
		if kv.result {
			color = aec.BlueF
		}
		fmt.Fprint(w, " ", base.With(color).Apply(kv.key+"="), kv.value)
	}

	if data.err != nil {
		t := errorType(data.err)
		if t == errIsCancelled || t == errIsDependencyFailed {
			fmt.Fprint(w, " ", base.With(aec.BlueF).Apply(string(t)))
		} else {
			fmt.Fprint(w, " ", base.With(aec.RedF).Apply("err="), base.With(aec.RedF).Apply(data.err.Error()))
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
	ID        string
	Name      string
	Created   time.Time
	State     string
	Completed *time.Time
}

func (c *ConsoleSink) redraw(t time.Time, flush bool) {
	if !c.rendering {
		return
	}

	var width uint
	var height uint
	if w, err := termios.TermSize(c.out.Fd()); err == nil {
		width = uint(w.Width)
		height = uint(w.Height)
	}

	c.outbuf.Reset()

	// Hide the cursor while re-rendering.
	fmt.Fprint(c.outbuf, aec.Hide)
	c.drawFrame(c.outbuf, t, width, height, flush)
	fmt.Fprint(c.outbuf, aec.Show)

	newFrame := c.outbuf.Bytes()

	if bytes.Equal(newFrame, c.lastFrame) {
		return
	}

	c.lastFrame = make([]byte, len(newFrame))
	copy(c.lastFrame, newFrame)

	c.out.Write(c.lastFrame)
}

func (c *ConsoleSink) drawFrame(out io.Writer, t time.Time, width, height uint, flush bool) {
	// Clear up everything we've previously written.
	if !DebugConsoleOutput {
		fmt.Fprint(out, aec.Up(c.previous))
		fmt.Fprint(out, aec.EraseDisplay(aec.EraseModes.Tail))
	}

	var running, anchored, waiting, completed, completedAnchors int
	var printableCompleted []*lineItem
	for _, r := range c.running {
		if r.data.state == actionRunning {
			if !r.data.indefinite {
				if r.data.anchorID != "" {
					anchored++
				} else {
					running++
				}
			}
		} else if r.data.state == actionWaiting {
			waiting++
		} else {
			hasError := (r.data.err != nil && errorType(r.data.err) == errIsRegular)
			shouldLog := LogActions && (DisplayWaitingActions || r.data.anchorID == "")

			if (shouldLog || hasError) && r.data.level <= c.maxLevel {
				printableCompleted = append(printableCompleted, r)
			}
			completed++
			if r.data.anchorID != "" {
				completedAnchors++
			}
		}
	}

	if LogActions && len(printableCompleted) > 0 {
		sort.Slice(printableCompleted, func(i, j int) bool {
			return printableCompleted[i].data.completed.Before(printableCompleted[j].data.completed)
		})

		for _, r := range printableCompleted {
			renderLine(out, r)
			if !r.data.started.IsZero() && !r.cached {
				if !r.data.started.Equal(r.data.created) {
					d := r.data.started.Sub(r.data.created)
					if d >= 1*time.Microsecond {
						fmt.Fprint(out, " ", aec.LightBlackF.Apply("waited="), timefmt.Format(d))
					}
				}

				d := r.data.completed.Sub(r.data.started)
				fmt.Fprint(out, " ", aec.LightBlackF.Apply("took="), timefmt.Format(d))
			}
			fmt.Fprintln(out)
		}
	}

	// Drain any pending logging message.
	var hdrBuf bytes.Buffer
	for _, block := range c.buffer {
		if block.name != "" && block.name != KnownStdout && block.name != KnownStderr {
			if block.cat == CatOutputUs {
				fmt.Fprint(&hdrBuf, usBar)
			} else {
				colorIndex := block.id.digest % uint64(len(toolBars))
				if includeToolIDs {
					fmt.Fprint(&hdrBuf, toolBars[colorIndex], ColorToolId.Apply(block.id.id)+" "+ColorToolName.Apply(block.name))
				} else {
					fmt.Fprint(&hdrBuf, toolBars[colorIndex], ColorToolName.Apply(block.name))
				}
			}
			for _, line := range block.lines {
				fmt.Fprintf(out, "%s %s\n", hdrBuf.Bytes(), line)
			}
			hdrBuf.Reset()
		} else {
			for _, line := range block.lines {
				fmt.Fprintf(out, "%s\n", line)
			}
		}
	}

	bufferCount := len(c.buffer)
	c.buffer = nil

	if c.debugOut != nil {
		var running []debugRunning

		for _, r := range c.running {
			var completed *time.Time
			if r.data.state == actionDone {
				completed = &r.data.completed
			}

			running = append(running, debugRunning{
				ID:        r.data.actionID,
				Name:      r.data.name,
				Created:   r.data.created,
				State:     string(r.data.state),
				Completed: completed,
			})
		}

		c.debugOut.Encode(debugData{
			Previous:    c.previous,
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
			if r.data.state.IsRunning() {
				newRunning = append(newRunning, r)
			}
		}
		c.running = newRunning
		c.recomputeTree()
	}

	c.previous = 0

	if len(c.stickyContent) > 0 {
		hdr := fmt.Sprintf("%s ", stickyBar)
		c.writeLineWithMaxW(out, width, hdr, "")
		for k, block := range c.stickyContent {
			if k > 0 && len(block.content) > 0 {
				c.writeLineWithMaxW(out, width, hdr, "")
			}
			for _, line := range block.content {
				c.writeLineWithMaxW(out, width, fmt.Sprintf("%s%s", hdr, line), "")
			}
		}
		c.writeLineWithMaxW(out, width, hdr, "")
	}

	if flush {
		return
	}

	if (waiting + running + anchored) == 0 {
		waitForIdle := c.waitForIdle
		c.waitForIdle = nil
		surpressBanner := false
		for _, f := range waitForIdle {
			if f() {
				surpressBanner = true
			}
		}
		if !surpressBanner {
			c.writeLineWithMaxW(out, width, fmt.Sprintf("[-] idle, %s.", c.idleLabel), "")
		}
		return
	}

	report := fmt.Sprintf("[+] %s", timefmt.Seconds(t.Sub(c.started)))
	report += fmt.Sprintf(" %s %s running", num(aec.GreenF, running), plural(running, "action", "actions"))
	if waiting > 0 {
		report += fmt.Sprintf(", %s waiting", num(aec.CyanF, waiting))
	}
	c.writeLineWithMaxW(out, width, report+".", "")

	maxDisplay := uint(4)
	if height > maxDisplay*2 {
		maxDisplay = height / 2
	}

	// Recurse through the line item tree.
	if !c.renderLineRec(out, width, c.root, t, " ", maxDisplay) {
		// Didn't have enough space for everything.
		c.writeLineWithMaxW(out, width, "", "(...)")
	}
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

func (c *ConsoleSink) renderLineRec(out io.Writer, width uint, n *node, t time.Time, inputPrefix string, maxDisplay uint) bool {
	prefix := inputPrefix + "=> "

	var lineb bytes.Buffer
	for _, id := range n.children {
		child := follow(c.nodes[id])

		if child.hidden {
			// If hidden, don't even go through it's children.
			continue
		}

		data := child.item.data

		skipRendering := data.level > c.maxLevel
		skipRendering = skipRendering || data.indefinite
		skipRendering = skipRendering || (!DisplayWaitingActions && data.state == actionWaiting)

		if skipRendering {
			// Recurse through the children of this line item, even if we don't render it.
			if !c.renderLineRec(out, width, child, t, inputPrefix, maxDisplay) {
				return false
			}
			continue
		}

		if c.previous >= maxDisplay {
			// Wanted to print a new line but have no spare space.
			return false
		}

		// Although this is not very efficient as we're thrashing strings, we need to make sure
		// we don't print more than one line, as that would disrupt the line acount we keep track
		// of to make for a smooth update in place (see use of c.previous above).
		// XXX precompute these lines as they don't change if the arguments don't change.
		lineb.Reset()

		if OutputActionID {
			fmt.Fprint(&lineb, aec.LightBlackF.Apply(" ["+data.actionID[:8]+"]"))
		}

		fmt.Fprint(&lineb, prefix)

		renderLine(&lineb, child.item)

		suffix := ""
		if data.state == actionRunning {
			d := t.Sub(data.started)
			suffix = " (" + timefmt.Format(d) + ")"
		} else if data.state == actionWaiting {
			suffix = " (waiting)"
		}

		c.writeLineWithMaxW(out, width, lineb.String(), suffix)

		if !c.renderLineRec(out, width, child, t, prefix, maxDisplay) {
			return false
		}
	}

	return true
}

func (c *ConsoleSink) writeLineWithMaxW(w io.Writer, width uint, line string, ensure string) {
	if width > 20 && (len(line)+len(ensure)) >= int(width-1) {
		line = line[:int(width-1)-len(ensure)]
	}
	fmt.Fprintln(w, line+ensure)
	c.previous++
}

func (c *ConsoleSink) Waiting(ra *RunningAction) {
	c.ch <- consoleEvent{ev: ra.data, progress: ra.progress}
}

func (c *ConsoleSink) Started(ra *RunningAction) {
	c.ch <- consoleEvent{ev: ra.data, progress: ra.progress}
}

func (c *ConsoleSink) Done(ra *RunningAction) {
	c.ch <- consoleEvent{ev: ra.data, results: ra.attachments.resultData}
}

func (c *ConsoleSink) Instant(ev *EventData) {
	c.ch <- consoleEvent{ev: *ev}
}

func (c *ConsoleSink) AttachmentsUpdated(actionID string, data *resultData) {
	if data != nil {
		c.ch <- consoleEvent{attachmentUpdatedForID: actionID, results: *data, progress: data.progress}
	}
}

func (c *ConsoleSink) WriteLines(id IdAndHash, name string, cat CatOutputType, lines [][]byte) {
	c.ch <- consoleEvent{output: consoleOutput{id: id, name: name, cat: cat, lines: lines}}
}

func (c *ConsoleSink) AllocateConsoleId() uint64 {
	return uint64(rand.Int63())
}

func SetIdleLabel(ctx context.Context, label string) func() {
	if console := ConsoleOf(SinkFrom(ctx)); console != nil {
		// XXX locking
		was := console.idleLabel
		console.idleLabel = label
		return func() { console.idleLabel = was }
	}

	return func() {}
}

func SetStickyContent(ctx context.Context, name string, content []byte) {
	if console := ConsoleOf(SinkFrom(ctx)); console != nil {
		var ev consoleEvent
		ev.setSticky.name = name
		ev.setSticky.contents = content
		console.ch <- ev
	}
}

func ConsoleOf(sink ActionSink) *ConsoleSink {
	if sink != nil {
		switch x := sink.(type) {
		case *ConsoleSink:
			return x
		case *statefulState:
			return ConsoleOf(x.parent)
		}
	}

	return nil
}

// Stops rendering actions. But only does so when an idle state is entered, and
// blocks until that point.
func EnterInputMode(ctx context.Context, prompt ...string) func() {
	c := ConsoleOf(SinkFrom(ctx))
	if c == nil {
		// No console, nothing to do.
		return func() {}
	}

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
