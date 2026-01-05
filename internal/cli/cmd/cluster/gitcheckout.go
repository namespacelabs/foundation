// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
)

func NewGitCheckoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "git-checkout",
		Short:  "Actions for git checkouts supporting Namespace Cache Volumes.",
		Hidden: true,
	}

	mirrorBaseDir := cmd.PersistentFlags().String("mirror_base_path", "", "the path of the mirror base directory")
	cmd.MarkPersistentFlagRequired("mirror_base_path")

	cmd.AddCommand(newUpdateSubmodulesCmd(mirrorBaseDir))

	return cmd
}

func newUpdateSubmodulesCmd(mirrorBaseDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-submodules",
		Short: "Updates git submodules using a Namespace git mirror.",
	}

	repositoryPath := cmd.Flags().String("repository_path", "", "the path of the repository to work in")
	cmd.MarkFlagRequired("repository_path")

	recurseSubmodules := cmd.Flags().Bool("recurse", false, "If true, will recursively update all submodules.")
	dissociate := cmd.Flags().Bool("dissociate", false, "If true, will dissociate all updated submodule checkouts from the cache.")
	depth := cmd.Flags().Int("depth", 0, "Truncate history to the specified number of commits.")
	filter := cmd.Flags().String("filter", "", "If specified, the given partial clone filter will be applied.")
	numWorkers := cmd.Flags().Int("workers", 4, "Number of workers for submodule fetch and update operations.")

	repoBufLen := cmd.Flags().Int("repo-buf-len", 1000, "max length of the pending repos buffer")
	cmd.Flags().MarkHidden("repo-buf-len")

	maxRecurseDepth := cmd.Flags().Int("max-recurse-depth", 20, "max depth of recursion into subdirectories")
	cmd.Flags().MarkHidden("max-recurse-depth")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		p := &processor{
			repoPath:          *repositoryPath,
			mirrorBaseDir:     *mirrorBaseDir,
			recurseSubmodules: *recurseSubmodules,
			dissociate:        *dissociate,
			depth:             *depth,
			filter:            *filter,
			numWorkers:        *numWorkers,
			repoBufLen:        *repoBufLen,
			maxRecurseDepth:   *maxRecurseDepth,
		}
		return p.updateSubmodules(ctx)
	})

	return cmd
}

type processor struct {
	// Path of the main repo to work on.
	repoPath string
	// Path of the base directory of the github mirror cache.
	mirrorBaseDir string
	// True if submodules of submodules should be checked out recursively.
	recurseSubmodules bool
	// True if the checked out submoudules should should not reference mirrorBaseDir after
	// this tool is done, i.e. pass --dissociate to git submodule update.
	dissociate bool
	// Truncate git history to this number of commits, i.e. pass --depth <depth> to git submodule update.
	// If 0, the option is omitted (all commits)
	depth int
	// The given partial clone filter will be applied to each submodule, i.e. pass --filter <filter> to git submodule update.
	// If empty, the option is omitted (all commits)
	filter string

	numWorkers      int
	repoBufLen      int
	maxRecurseDepth int

	processRepoJobQueue         chan processRepoJob
	doProcessRepoJobQueue       chan processRepoJob
	processRepoDoneNotification chan processRepoJob

	ensureMirrorJobQueue           chan ensureMirrorJob
	doEnsureMirrorJobQueue         chan doEnsureMirrorJob
	doEnsureMirrorDoneNotification chan doEnsureMirrorResult

	// Overall done signal: done when closed.
	// Multiple "worker" goroutines can use this as a signal to exit when no more work can arrive.
	doneSignal chan struct{}
}

type submodule struct {
	// Name of the entry in .gitmodules
	configKey string
	// Relative path where the submodule is checked out in the repo
	relativePath string
	// Remote repository url
	remoteUrl string
}

type processRepoJob struct {
	repoPath       string
	recursionDepth int
}

type ensureMirrorJob struct {
	submod submodule

	resultChan chan ensureMirrorResult
}

type ensureMirrorResult struct {
	submod    submodule
	mirrorDir string
	err       error
}

type doEnsureMirrorJob struct {
	remoteUrl string
	mirrorDir string
}

type doEnsureMirrorResult struct {
	remoteUrl string
	mirrorDir string
	err       error
}

func (p *processor) updateSubmodules(ctx context.Context) error {
	if p.numWorkers < 1 || p.numWorkers > 8 {
		return fmt.Errorf("needs 1..8 workers")
	}
	if p.repoBufLen < 1 {
		return fmt.Errorf("needs repo-buf-len >= 1")
	}

	if p.mirrorBaseDir == "" {
		return fmt.Errorf("nsc git-checkout update-submodules requires Git mirror to be enabled on the Namespace runner.")
	}
	// The processing of nested submodules requires this to be an absolute path.
	// This is because it's using `git -C <dir>` (becuase the submodule commands don't respect --work-tree)
	// so relative paths would change in meaning.
	absMirrorBaseDir, err := filepath.Abs(p.mirrorBaseDir)
	if err != nil {
		return fmt.Errorf("could not convert '%s' to an absolute path: %v", p.mirrorBaseDir, err)
	}
	p.mirrorBaseDir = absMirrorBaseDir
	if err := checkIsDir(p.mirrorBaseDir); err != nil {
		return fmt.Errorf("Didn't find directory under '%s': %v", p.mirrorBaseDir, err)
	}

	p.processRepoJobQueue = make(chan processRepoJob)
	p.doProcessRepoJobQueue = make(chan processRepoJob, p.repoBufLen)
	p.processRepoDoneNotification = make(chan processRepoJob)

	p.ensureMirrorJobQueue = make(chan ensureMirrorJob)
	p.doEnsureMirrorJobQueue = make(chan doEnsureMirrorJob, p.repoBufLen)
	p.doEnsureMirrorDoneNotification = make(chan doEnsureMirrorResult)

	p.doneSignal = make(chan struct{})

	if err := checkIsDir(p.repoPath); err != nil {
		return err
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return p.processRepoCoordinator(groupCtx)
	})
	group.Go(func() error {
		return p.ensureMirrorCoordinator(groupCtx)
	})
	for i := 0; i < p.numWorkers; i++ {
		group.Go(func() error {
			return p.workerForProcessRepo(groupCtx)
		})
		group.Go(func() error {
			return p.workerForDoEnsureMirror(groupCtx)
		})
	}

	p.scheduleProcessRepo(ctx, p.repoPath, 0)

	return group.Wait()
}

func (p *processor) scheduleProcessRepo(ctx context.Context, repoPath string, recursionDepth int) {
	fmt.Fprintf(console.Debug(ctx), "N%d: In %s: scheduled processing\n", recursionDepth, repoPath)

	p.processRepoJobQueue <- processRepoJob{
		repoPath:       repoPath,
		recursionDepth: recursionDepth,
	}
}

// - Forwards processSubmoduleJob jobs to workers
// - Keeps track of repo paths that are being processed
// - When that becomes an empty set (apart from the startup case), signals doneSignal
func (p *processor) processRepoCoordinator(ctx context.Context) error {
	activeRepoPaths := map[string]struct{}{}

	for {
		select {
		case job := <-p.processRepoJobQueue:
			fmt.Fprintf(console.Info(ctx), "N%d: Entering %s\n", job.recursionDepth, job.repoPath)
			activeRepoPaths[job.repoPath] = struct{}{}
			select {
			case p.doProcessRepoJobQueue <- job:
			default:
				return fmt.Errorf("reached repo buf queue length in processRepo")
			}

		case job := <-p.processRepoDoneNotification:
			fmt.Fprintf(console.Info(ctx), "N%d: Leaving %s\n", job.recursionDepth, job.repoPath)
			delete(activeRepoPaths, job.repoPath)
			if len(activeRepoPaths) == 0 {
				close(p.doneSignal)
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()

		}
	}
}

func (p *processor) workerForProcessRepo(ctx context.Context) error {
	for {
		select {
		case work := <-p.doProcessRepoJobQueue:
			err := p.doProcessRepo(ctx, work)
			if err != nil {
				return err
			}

		case <-p.doneSignal:
			return nil

		case <-ctx.Done():
			return ctx.Err()

		}
	}
}

func (p *processor) doProcessRepo(ctx context.Context, job processRepoJob) error {
	if job.recursionDepth > p.maxRecurseDepth {
		return fmt.Errorf("Reached max nesting level: %d", job.recursionDepth)
	}

	submodules, err := getSubmodules(ctx, job.repoPath)
	if err != nil {
		return err
	}

	// NSL-3898: get origin repository to be able to resolve potential relative submodule URL
	origin, err := getRepoRemoteOrigin(ctx, job.repoPath)
	if err != nil {
		return err
	}

	submodules, err = resolveRelativeRemoteUrls(submodules, origin)
	if err != nil {
		return err
	}

	mirrorReadyChan := make(chan ensureMirrorResult, len(submodules))
	for _, submod := range submodules {
		fmt.Fprintf(console.Info(ctx), "N%d: In %s: Found submodule %s -> %s\n", job.recursionDepth, job.repoPath, submod.relativePath, submod.remoteUrl)
		p.scheduleEnsureMirror(submod, mirrorReadyChan)
	}

	// The actual "update" operations for submodules must be performed sequentially
	for i := 0; i < len(submodules); i++ {
		select {
		case ensureMirrorResult := <-mirrorReadyChan:
			if ensureMirrorResult.err != nil {
				return ensureMirrorResult.err
			}
			err := p.doUpdateSubmodule(ctx, job.repoPath, job.recursionDepth, ensureMirrorResult.submod, ensureMirrorResult.mirrorDir)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	fmt.Fprintf(console.Debug(ctx), "N%d: In %s: done processing\n", job.recursionDepth, job.repoPath)
	select {
	case p.processRepoDoneNotification <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *processor) doUpdateSubmodule(ctx context.Context, repoPath string, recursionDepth int, submod submodule, mirrorDir string) error {
	fmt.Fprintf(console.Info(ctx), "N%d: In %s: Update %s from mirror of %s\n", recursionDepth, repoPath, submod.relativePath, submod.remoteUrl)

	// Actually get the submodule, using the mirror as reference
	submoduleUpdateArgs := []string{"submodule", "update", "--init", "--reference", mirrorDir}
	if p.dissociate {
		submoduleUpdateArgs = append(submoduleUpdateArgs, "--dissociate")
	}
	if p.depth > 0 {
		submoduleUpdateArgs = append(submoduleUpdateArgs, "--depth", strconv.Itoa(p.depth))
	}
	if p.filter != "" {
		submoduleUpdateArgs = append(submoduleUpdateArgs, "--filter", p.filter)
	}
	submoduleUpdateArgs = append(submoduleUpdateArgs, submod.relativePath)
	cmd := inRepoGit(repoPath, submoduleUpdateArgs...)
	_, err := runAndPrintIfFails(ctx, cmd)
	if err != nil {
		return fmt.Errorf("could not update submodule '%s': %v", submod.relativePath, err)
	}

	if p.recurseSubmodules {
		recursePath := filepath.Join(repoPath, submod.relativePath)
		p.scheduleProcessRepo(ctx, recursePath, recursionDepth+1)
	}

	return nil
}

func (p *processor) scheduleEnsureMirror(submod submodule, resultChan chan ensureMirrorResult) {
	p.ensureMirrorJobQueue <- ensureMirrorJob{submod: submod, resultChan: resultChan}
}

func (p *processor) ensureMirrorCoordinator(ctx context.Context) error {
	type mirrorStatus int

	const (
		MIRROR_STATUS_PENDING mirrorStatus = iota
		MIRROR_STATUS_FETCH_SCHEDULED
		MIRROR_STATUS_FETCH_COMPLETE
	)

	type pendingNotification struct {
		channel chan ensureMirrorResult
		submod  submodule
	}

	type mirrorInfo struct {
		status mirrorStatus
		// Notifications to be sent when this mirror is up to date.
		pendingNotifications []pendingNotification
	}

	mirrors := map[string]*mirrorInfo{}

	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case job := <-p.ensureMirrorJobQueue:
			submod := job.submod
			mirrorDir := getMirrorDir(p.mirrorBaseDir, submod)

			mirror, ok := mirrors[submod.remoteUrl]
			if !ok {
				mirror = &mirrorInfo{}
				mirrors[submod.remoteUrl] = mirror
			}

			switch mirror.status {
			case MIRROR_STATUS_FETCH_COMPLETE:
				fmt.Fprintf(console.Debug(ctx), "mirror for %s at %s is up to date\n", submod.remoteUrl, mirrorDir)
				// The mirror is up to date
				job.resultChan <- ensureMirrorResult{submod: submod, mirrorDir: mirrorDir}
			case MIRROR_STATUS_FETCH_SCHEDULED:
				fmt.Fprintf(console.Debug(ctx), "mirror for %s at %s has a fetch running\n", submod.remoteUrl, mirrorDir)
				mirror.pendingNotifications = append(mirror.pendingNotifications, pendingNotification{channel: job.resultChan, submod: submod})
			case MIRROR_STATUS_PENDING:
				fmt.Fprintf(console.Debug(ctx), "mirror for %s at %s is pending, trigger fetch\n", submod.remoteUrl, mirrorDir)
				mirror.pendingNotifications = append(mirror.pendingNotifications, pendingNotification{channel: job.resultChan, submod: submod})

				select {
				case p.doEnsureMirrorJobQueue <- doEnsureMirrorJob{remoteUrl: submod.remoteUrl, mirrorDir: mirrorDir}:
				case <-ctx.Done():
					return ctx.Err()
				default:
					return fmt.Errorf("reached repo buf queue length in ensureMirror")
				}
				mirror.status = MIRROR_STATUS_FETCH_SCHEDULED
			}

		case result := <-p.doEnsureMirrorDoneNotification:
			if result.err == nil {
				fmt.Fprintf(console.Info(ctx), "fetching or cloning mirror for %s at %s SUCCESS\n", result.remoteUrl, result.mirrorDir)
			} else {
				fmt.Fprintf(console.Info(ctx), "fetching or cloning mirror for %s at %s FAILURE: %v\n", result.remoteUrl, result.mirrorDir, result.err)
			}

			mirror, ok := mirrors[result.remoteUrl]
			if !ok {
				panic("no mirror info")
			}

			mirror.status = MIRROR_STATUS_FETCH_COMPLETE
			for _, notification := range mirror.pendingNotifications {
				select {
				case notification.channel <- ensureMirrorResult{
					submod:    notification.submod,
					mirrorDir: result.mirrorDir,
					err:       result.err,
				}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			clear(mirror.pendingNotifications)

		case <-ticker.C:
			processing := []string{}
			for remoteUrl, mirror := range mirrors {
				if mirror.status == MIRROR_STATUS_FETCH_SCHEDULED {
					processing = append(processing, remoteUrl)
				}
			}
			if len(processing) > 0 {
				sort.Strings(processing)
				fmt.Fprintf(console.Debug(ctx), "still fetching %s\n", strings.Join(processing, ", "))
			}

		case <-p.doneSignal:
			return nil

		case <-ctx.Done():
			return ctx.Err()

		}
	}
}

func (p *processor) workerForDoEnsureMirror(ctx context.Context) error {
	for {
		select {
		case work := <-p.doEnsureMirrorJobQueue:
			if err := p.doEnsureMirror(ctx, work); err != nil {
				return err
			}

		case <-p.doneSignal:
			return nil

		case <-ctx.Done():
			return ctx.Err()

		}
	}
}

func (p *processor) doEnsureMirror(ctx context.Context, job doEnsureMirrorJob) error {
	fmt.Fprintf(console.Info(ctx), "fetching or cloning mirror for %s at %s\n", job.remoteUrl, job.mirrorDir)
	err := ensureMirrorWork(ctx, job.remoteUrl, job.mirrorDir)

	select {
	case p.doEnsureMirrorDoneNotification <- doEnsureMirrorResult{remoteUrl: job.remoteUrl, mirrorDir: job.mirrorDir, err: err}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func ensureMirrorWork(ctx context.Context, remoteUrl string, mirrorDir string) error {
	if err := os.MkdirAll(mirrorDir, os.ModePerm); err != nil {
		return fmt.Errorf("can not create '%s' for '%s': %v", mirrorDir, remoteUrl, err)
	}

	if isMirrorRepo(mirrorDir) {
		// Make sure the mirror is up to date
		cmd := inRepoGit(mirrorDir, "fetch", "--no-recurse-submodules", "origin")
		_, err := runAndPrintIfFails(ctx, cmd)
		if err != nil {
			return fmt.Errorf("could not git fetch '%s' in '%s': %v", remoteUrl, mirrorDir, err)
		}
	} else {
		// Create new mirror
		cmd := exec.Command("git", "clone", "--mirror", "--", remoteUrl, mirrorDir)
		_, err := runAndPrintIfFails(ctx, cmd)
		if err != nil {
			return fmt.Errorf("could not git clone '%s' to '%s': %v", remoteUrl, mirrorDir, err)
		}
	}
	return nil
}

func isMirrorRepo(mirrorDir string) bool {
	// Mirror repos are bare repositories, so check for that.
	// This not failing implies that mirrorDir is a git repository in the first place.
	err := inRepoGit(mirrorDir, "rev-parse", "--is-bare-repository").Run()
	return err == nil
}

func getMirrorDir(mirrorBaseDir string, mod submodule) string {
	charsToReplace := regexp.MustCompile("[^[:alnum:]-_\\.]")
	key := "submod-" + charsToReplace.ReplaceAllString(mod.remoteUrl, "_")
	return filepath.Join(mirrorBaseDir, "v2", key)
}

func checkIsDir(repoPath string) error {
	stat, err := os.Stat(repoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path '%s' does not exist", repoPath)
		} else {
			return fmt.Errorf("i/o error when checking '%s'", repoPath)
		}
	}
	if !stat.IsDir() {
		return fmt.Errorf("path '%s' is not a directory", repoPath)
	}
	return nil
}

// Executes "git" as if it ran in "repoPath" in a different way :-)
// Becuase git submodule doesn't seem to support --work-tree.
func inRepoGit(repoPath string, args ...string) *exec.Cmd {
	allArgs := append(
		[]string{"-C", repoPath},
		args...)

	return exec.Command("git", allArgs...)
}

func getRepoRemoteOrigin(ctx context.Context, repoPath string) (string, error) {
	cmd := inRepoGit(repoPath, "config", "--get", "remote.origin.url")
	output, err := runAndPrintIfFails(ctx, cmd)
	if err != nil {
		return "", err
	}

	return strings.Trim(string(output), string([]rune{'\n', ' '})), nil
}

func resolveRelativeRemoteUrls(submodules []submodule, originRemoteUrl string) ([]submodule, error) {
	for i, sub := range submodules {
		if isRelativeUrl(sub.remoteUrl) {
			resolvedUrl, err := resolveRelativeRemoteUrl(sub.remoteUrl, originRemoteUrl)
			if err != nil {
				return nil, err
			}

			sub.remoteUrl = resolvedUrl.URL()
			submodules[i] = sub
		}
	}

	return submodules, nil
}

func getSubmodules(ctx context.Context, repoPath string) ([]submodule, error) {
	cmd := inRepoGit(repoPath, "config", "--file", ".gitmodules", "--get-regexp", "submodule\\.")
	fmt.Fprintf(console.Debug(ctx), "exec: %s\n", strings.Join(cmd.Args, " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return []submodule{}, err
	}
	scanner := bufio.NewScanner(stdout)
	err = cmd.Start()
	if err != nil {
		return []submodule{}, err
	}

	submoduleMap := map[string]*submodule{}
	// For each submodule, this should produce at least
	// submodule.<submodule-config-key>.path <path-in-repo>
	// submodule.<submodule-config-key>.url <remote-url>
	for scanner.Scan() {
		line := scanner.Text()
		ok, key, value := parseGitConfigKeyValue(line)
		if !ok {
			return []submodule{}, fmt.Errorf("could not parse git config output line '%s'", line)
		}
		ok, submoduleConfigKey, submoduleAttrName := parseSubmoduleConfigKey(key)
		if !ok {
			return []submodule{}, fmt.Errorf("could not parse git config submodule line '%s'", line)
		}

		entry, ok := submoduleMap[submoduleConfigKey]
		if !ok {
			entry = &submodule{}
			entry.configKey = submoduleConfigKey
			submoduleMap[submoduleConfigKey] = entry
		}

		switch submoduleAttrName {
		case "path":
			entry.relativePath = value
		case "url":
			entry.remoteUrl = value
		}
	}
	if scanner.Err() != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return []submodule{}, scanner.Err()
	}
	if err := cmd.Wait(); err != nil {
		return []submodule{}, scanner.Err()
	}

	if err := filterSubmodules(ctx, repoPath, submoduleMap); err != nil {
		return []submodule{}, err
	}

	res := make([]submodule, 0, len(submoduleMap))
	for _, submodule := range submoduleMap {
		res = append(res, *submodule)
	}
	return res, nil
}

// Parses a "git config" output line such as
// key value
// (e.g. "submodule.<submodule-config-key>.path <path-in-repo>")
// returning (ok, key, value)
func parseGitConfigKeyValue(line string) (bool, string, string) {
	separator := strings.IndexByte(line, ' ')
	if separator < 0 {
		return false, "", ""
	}
	key := line[0:separator]
	value := line[separator+1:]

	return true, key, value
}

// Parses a "git config" submodule key such as "submodule.<submodule-config-key>.subkey"
// returning (ok, submodule-config-key, subkey)
func parseSubmoduleConfigKey(key string) (bool, string, string) {
	split := strings.Split(key, ".")
	if len(split) != 3 {
		return false, "", ""
	}
	if split[0] != "submodule" {
		return false, "", ""
	}
	return true, split[1], split[2]
}

func runAndPrintIfFails(ctx context.Context, cmd *exec.Cmd) (string, error) {
	fmt.Fprintf(console.Info(ctx), "exec: %s\n", strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(console.Errors(ctx), "failed: %s\n", strings.Join(cmd.Args, " "))
		fmt.Fprintln(console.Errors(ctx), string(output))
		return string(output), err
	}
	return string(output), nil
}

// Filters submoduleMap: Only retains entries that point to a "commit" git object.
// submoduleMap is modified in place.
func filterSubmodules(ctx context.Context, repoPath string, submoduleMap map[string]*submodule) error {
	// .gitmodules can contain entries which are ignored by regular git submodule commands
	// Specifically if there is no corresponding "commit" object.
	// git ls-tree -d  HEAD submodule1/ nscloud-checkout-action/
	//

	args := []string{
		"ls-tree",
		// Don't recurse (in case this finds a tree object)
		"-d",
		// Configure output format in a way where parseGitConfigKeyValue can be used.
		"--format=%(objecttype) %(path)",
		"HEAD"}
	for _, submodule := range submoduleMap {
		args = append(args, submodule.relativePath)
	}

	cmd := inRepoGit(repoPath, args...)
	fmt.Fprintf(console.Debug(ctx), "exec: %s\n", strings.Join(cmd.Args, " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	err = cmd.Start()
	if err != nil {
		return err
	}

	goodSubmodules := map[string]struct{}{}
	for scanner.Scan() {
		// Only accept "commit" object types.
		line := scanner.Text()
		// We configured --format specifically so this can be used
		ok, objectType, path := parseGitConfigKeyValue(line)
		if !ok {
			return fmt.Errorf("could not parse git ls-tree output line '%s'", line)
		}

		if objectType != "commit" {
			fmt.Fprintf(console.Warnings(ctx), "Found non-commit object: %s (type %s)\n", line, objectType)
			continue
		}

		// .gitmodules path can not contain a trailing slash or .. so
		// matching on path as a string is valid.
		goodSubmodules[path] = struct{}{}
	}

	if scanner.Err() != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return scanner.Err()
	}
	if err := cmd.Wait(); err != nil {
		return scanner.Err()
	}

	for key, submod := range submoduleMap {
		_, ok := goodSubmodules[submod.relativePath]
		if !ok {
			fmt.Fprintf(console.Warnings(ctx), "Submodule in %s did not point to a git object\n", submod.relativePath)
			delete(submoduleMap, key)
			continue
		}
		delete(goodSubmodules, submod.relativePath)
	}

	for unexpected, _ := range goodSubmodules {
		return fmt.Errorf("Got unexpected info about obj %s\n", unexpected)
	}

	return nil
}
