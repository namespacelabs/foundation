// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/mattn/go-isatty"
	"golang.org/x/sync/semaphore"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	defaultChunkSize   = 4 * 1024 * 1024 // 4MB chunks
	defaultConcurrent  = 4               // 4 concurrent downloads
	maxRetriesPerChunk = 10              // Can survive ~60s of downtime with backoff
	localStateVersion  = 1
)

type Options struct {
	ChunkSize       int64
	Concurrent      int
	Resume          bool
	OnProgress      func(downloaded, total int64)
	ResolveURL      func(ctx context.Context) (string, error)
	SuppressConsole bool
	sleepFunc       func(context.Context, time.Duration) error
}

type downloadState struct {
	Version         int              `json:"version"`
	ChunkSize       int64            `json:"chunk_size"`
	ChunksDone      []int64          `json:"chunks_done"`
	Digests         map[int64]string `json:"digests,omitempty"`
	DownloadedBytes int64            `json:"downloaded_bytes"`
}

type localState struct {
	stateFile string
	state     *downloadState
	mu        sync.Mutex
}

func (ls *localState) finishedChunk(chunkNum, chunkBytes int64, digest string) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.state.ChunksDone = append(ls.state.ChunksDone, chunkNum)
	ls.state.DownloadedBytes += chunkBytes
	slices.Sort(ls.state.ChunksDone)

	if digest != "" {
		if ls.state.Digests == nil {
			ls.state.Digests = map[int64]string{}
		}
		ls.state.Digests[chunkNum] = digest
	}

	return ls.save()
}

func (ls *localState) isFinished(chunkNum int64) bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return slices.Contains(ls.state.ChunksDone, chunkNum)
}

func (ls *localState) save() error {
	if ls.stateFile == "" {
		return nil
	}

	data, err := json.Marshal(ls.state)
	if err != nil {
		return err
	}

	tmpPath := ls.stateFile + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, ls.stateFile)
}

func (ls *localState) completed() error {
	if ls.stateFile == "" {
		return nil
	}
	return os.Remove(ls.stateFile)
}

func newState(stateFile string, chunkSize int64) *localState {
	return &localState{
		stateFile: stateFile,
		state: &downloadState{
			Version:    localStateVersion,
			ChunkSize:  chunkSize,
			ChunksDone: []int64{},
		},
	}
}

func loadState(stateFile string, chunkSize int64) (*localState, bool, error) {
	if stateFile == "" {
		return newState("", chunkSize), true, nil
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return newState(stateFile, chunkSize), true, nil
		}
		return nil, false, err
	}

	var state downloadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false, err
	}

	if state.Version != localStateVersion || chunkSize != state.ChunkSize {
		return newState(stateFile, chunkSize), true, nil
	}

	return &localState{stateFile: stateFile, state: &state}, false, nil
}

func Download(ctx context.Context, destPath string, opts Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if opts.ChunkSize == 0 {
		opts.ChunkSize = defaultChunkSize
	}
	if opts.Concurrent == 0 {
		opts.Concurrent = defaultConcurrent
	}
	if opts.ResolveURL == nil {
		return fnerrors.New("ResolveURL is required")
	}
	if opts.sleepFunc == nil {
		opts.sleepFunc = defaultSleep
	}

	resolvedURL, err := opts.ResolveURL(ctx)
	if err != nil {
		return fnerrors.Newf("failed to resolve URL: %w", err)
	}

	resp, err := http.Head(resolvedURL)
	if err != nil {
		return fnerrors.Newf("failed to HEAD %s: %w", resolvedURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fnerrors.Newf("HEAD request failed with status %d", resp.StatusCode)
	}

	contentLength := resp.ContentLength
	if contentLength <= 0 {
		return downloadSingleStream(ctx, opts, destPath)
	}

	if contentLength < opts.ChunkSize {
		return downloadSingleStream(ctx, opts, destPath)
	}

	acceptsRanges := resp.Header.Get("Accept-Ranges") == "bytes"
	if !acceptsRanges {
		return downloadSingleStream(ctx, opts, destPath)
	}

	var stateFile string
	var downloadFile string

	if opts.Resume {
		stateFile = destPath + ".state"
		downloadFile = destPath + ".download"
	} else {
		downloadFile = filepath.Join(filepath.Dir(destPath), "."+filepath.Base(destPath)+".tmp")
	}

	state, newState, err := loadState(stateFile, opts.ChunkSize)
	if err != nil {
		return fnerrors.Newf("failed to load resume state: %w", err)
	}

	totalChunks := (contentLength + opts.ChunkSize - 1) / opts.ChunkSize

	if !newState && state.state.DownloadedBytes > 0 {
		remaining := contentLength - state.state.DownloadedBytes
		fmt.Fprintf(os.Stderr, "Resuming download: %s downloaded, %s remaining\n",
			humanize.IBytes(uint64(state.state.DownloadedBytes)),
			humanize.IBytes(uint64(remaining)))
	}

	if newState {
		if err := state.save(); err != nil {
			return fnerrors.Newf("failed to save initial state: %w", err)
		}

		f, err := os.OpenFile(downloadFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if err != nil {
			return fnerrors.Newf("failed to create download file: %w", err)
		}

		if err := f.Truncate(contentLength); err != nil {
			f.Close()
			return fnerrors.Newf("failed to allocate file: %w", err)
		}

		if err := f.Close(); err != nil {
			return fnerrors.Newf("failed to close download file: %w", err)
		}
	}

	var downloadedBytes atomic.Int64
	var activeStreams atomic.Int64
	downloadedBytes.Store(state.state.DownloadedBytes)

	progressCtx, cancelProgress := context.WithCancel(ctx)
	defer cancelProgress()

	progressDone := make(chan struct{})
	isConsole := !opts.SuppressConsole && isatty.IsTerminal(os.Stderr.Fd())

	if isConsole {
		go reportProgressWithSpinner(progressCtx, cancel, &downloadedBytes, &activeStreams, contentLength, progressDone)
	} else {
		go reportProgress(progressCtx, &downloadedBytes, &activeStreams, contentLength, progressDone)
	}

	downloadErr := runShardedTask(ctx, atMost(opts.Concurrent), shardedTask{
		ShardCount:      totalChunks,
		RetriesPerShard: maxRetriesPerChunk,
		SleepFunc:       opts.sleepFunc,
		RunTask: func(ctx context.Context, shard int64) error {
			return downloadChunk(ctx, downloadFile, shard, contentLength, state, &downloadedBytes, &activeStreams, opts)
		},
	})

	cancelProgress()
	<-progressDone

	if downloadErr != nil {
		return downloadErr
	}

	if err := os.Rename(downloadFile, destPath); err != nil {
		return fnerrors.Newf("failed to rename download file: %w", err)
	}

	return state.completed()
}

type acquireFunc func(context.Context) (func(), error)

func defaultSleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func atMost(n int) acquireFunc {
	sem := semaphore.NewWeighted(int64(n))

	return func(ctx context.Context) (func(), error) {
		if err := sem.Acquire(ctx, 1); err != nil {
			return nil, err
		}

		return func() {
			sem.Release(1)
		}, nil
	}
}

type shardedTask struct {
	ShardCount      int64
	RetriesPerShard int
	RunTask         func(ctx context.Context, shard int64) error
	SleepFunc       func(context.Context, time.Duration) error
}

func runShardedTask(ctx context.Context, acquire acquireFunc, task shardedTask) error {
	var wg sync.WaitGroup
	var errOnce sync.Once
	var finalErr error

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := (func() error {
		for i := int64(0); i < task.ShardCount; i++ {
			shard := i

			release, err := acquire(ctx)
			if err != nil {
				return err
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				defer release()

				attempts := 0
				for {
					if err := task.RunTask(ctx, shard); err != nil {
						attempts++
						if attempts >= task.RetriesPerShard {
							errOnce.Do(func() {
								finalErr = err
							})
							cancel()
							break
						}

						fmt.Fprintf(os.Stderr, "chunk %d failed: %v, retrying...\n", shard, err)

						backoff := time.Duration(1<<uint(attempts-1)) * time.Second
						if backoff > 10*time.Second {
							backoff = 10 * time.Second
						}

						if err := task.SleepFunc(ctx, backoff); err != nil {
							return
						}
					} else {
						break
					}
				}
			}()
		}

		return nil
	})(); err != nil {
		errOnce.Do(func() {
			finalErr = err
		})
	}

	wg.Wait()

	return finalErr
}

func downloadChunk(ctx context.Context, outputFile string, chunkNum, totalSize int64, state *localState, downloadedBytes, activeStreams *atomic.Int64, opts Options) error {
	if state.isFinished(chunkNum) {
		return nil
	}

	activeStreams.Add(1)
	defer activeStreams.Add(-1)

	start := chunkNum * opts.ChunkSize
	chunkSize := opts.ChunkSize
	if (start + chunkSize) > totalSize {
		chunkSize = totalSize - start
	}
	end := start + chunkSize - 1

	url, err := opts.ResolveURL(ctx)
	if err != nil {
		return fnerrors.Newf("failed to resolve URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fnerrors.Newf("unexpected status %d for chunk download", resp.StatusCode)
	}

	outFile, err := os.OpenFile(outputFile, os.O_RDWR, 0644)
	if err != nil {
		return fnerrors.Newf("error opening output file: %w", err)
	}
	defer outFile.Close()

	if _, err := outFile.Seek(start, 0); err != nil {
		return fnerrors.Newf("error seeking in output file: %w", err)
	}

	h := sha256.New()
	out := io.MultiWriter(outFile, h)

	buf := make([]byte, 32*1024)
	written := int64(0)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			written += int64(n)
			downloadedBytes.Add(int64(n))

			if opts.OnProgress != nil {
				opts.OnProgress(downloadedBytes.Load(), totalSize)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	if written != chunkSize {
		return fnerrors.Newf("expected %d bytes, got %d", chunkSize, written)
	}

	if err := outFile.Close(); err != nil {
		return fnerrors.Newf("failed to close file: %w", err)
	}

	hashBytes := h.Sum(nil)
	digest := "sha256:" + hex.EncodeToString(hashBytes)

	return state.finishedChunk(chunkNum, written, digest)
}

func downloadSingleStream(ctx context.Context, opts Options, destPath string) error {
	url, err := opts.ResolveURL(ctx)
	if err != nil {
		return fnerrors.Newf("failed to resolve URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fnerrors.Newf("failed to GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fnerrors.Newf("GET request failed with status %d", resp.StatusCode)
	}

	tmpDir := filepath.Dir(destPath)
	tmpFile := filepath.Join(tmpDir, "."+filepath.Base(destPath)+".tmp")

	f, err := os.Create(tmpFile)
	if err != nil {
		return fnerrors.Newf("failed to create temp file: %w", err)
	}
	defer f.Close()

	var totalDownloaded int64
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			totalDownloaded += int64(n)

			if opts.OnProgress != nil {
				opts.OnProgress(totalDownloaded, resp.ContentLength)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	if err := f.Close(); err != nil {
		return fnerrors.Newf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpFile, destPath); err != nil {
		return fnerrors.Newf("failed to rename temp file: %w", err)
	}

	return nil
}

func ComputeDigest(ctx context.Context, filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	hashBytes := h.Sum(nil)
	return "sha256:" + hex.EncodeToString(hashBytes), nil
}

type progressMsg struct{}
type progressModel struct {
	spinner         spinner.Model
	downloadedBytes *atomic.Int64
	activeStreams   *atomic.Int64
	totalSize       int64
	lastBytes       int64
	lastTime        time.Time
	cancelRequested bool
	expertMode      bool
}

var mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#909090", Dark: "#626262"})
var spinnerColor = lipgloss.Color("205")

func newSpinnerModel() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(spinnerColor)
	return s
}

func (m progressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return progressMsg{}
	})
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelRequested = true
			return m, tea.Quit
		case "x":
			m.expertMode = !m.expertMode
			return m, nil
		}
		return m, nil
	case progressMsg:
		return m, tickCmd()
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m progressModel) View() string {
	currentBytes := m.downloadedBytes.Load()
	now := time.Now()
	elapsed := now.Sub(m.lastTime).Seconds()

	var rate float64
	if elapsed > 0 {
		rate = float64(currentBytes-m.lastBytes) / elapsed
		m.lastBytes = currentBytes
		m.lastTime = now
	}

	percentage := float64(currentBytes) / float64(m.totalSize) * 100

	desc := fmt.Sprintf("%.1f%% (%s/%s) @ %s/s",
		percentage,
		humanize.IBytes(uint64(currentBytes)),
		humanize.IBytes(uint64(m.totalSize)),
		humanize.IBytes(uint64(rate)))

	if m.expertMode {
		active := m.activeStreams.Load()
		desc += fmt.Sprintf(" (over %d streams)", active)
	}

	return fmt.Sprintf("%s %s", m.spinner.View(), mutedStyle.Render(desc))
}

func reportProgressWithSpinner(ctx context.Context, cancelDownload context.CancelFunc, downloadedBytes, activeStreams *atomic.Int64, totalSize int64, done chan struct{}) {
	defer close(done)

	model := progressModel{
		spinner:         newSpinnerModel(),
		downloadedBytes: downloadedBytes,
		activeStreams:   activeStreams,
		totalSize:       totalSize,
		lastBytes:       downloadedBytes.Load(),
		lastTime:        time.Now(),
	}

	p := tea.NewProgram(model, tea.WithOutput(os.Stderr), tea.WithContext(ctx))

	finalModel, _ := p.Run()

	if m, ok := finalModel.(progressModel); ok && m.cancelRequested {
		cancelDownload()
	}
}

func reportProgress(ctx context.Context, downloadedBytes, activeStreams *atomic.Int64, totalSize int64, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	var lastBytes int64 = downloadedBytes.Load()
	var lastTime time.Time = time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			currentBytes := downloadedBytes.Load()
			elapsed := now.Sub(lastTime).Seconds()
			if elapsed == 0 {
				continue
			}

			rate := float64(currentBytes-lastBytes) / elapsed
			lastBytes = currentBytes
			lastTime = now

			percentage := float64(currentBytes) / float64(totalSize) * 100
			active := activeStreams.Load()

			fmt.Fprintf(os.Stderr, "Downloading: %.1f%% (%s/%s) @ %s/s (over %d streams)\n",
				percentage,
				humanize.IBytes(uint64(currentBytes)),
				humanize.IBytes(uint64(totalSize)),
				humanize.IBytes(uint64(rate)),
				active)
		}
	}
}
