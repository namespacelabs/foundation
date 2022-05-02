// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"filippo.io/age"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/maketarfs"
)

const (
	// Public key used to encrypt bundles before uploading to the bundle service.
	// Generate with `age-keygen` and needs to be kept in sync with the private
	// internal key available only to foundation core devs.
	publicKey = "age1ngp9m4wrhq4zvc2redr7jm8gat0qnkue4dfsklqdxg5yn7w0xsqqwp3jgw"
)

// Bundle is a fs with an associated timestamp.
type Bundle struct {
	fsys      fnfs.ReadWriteFS
	Timestamp time.Time

	// Guards writes to the bundle.
	mu sync.Mutex
}

// Invocation information associated with a bundle.
type InvocationInfo struct {
	Command string
	Os      string
	Arch    string
	NumCpu  int
	Version version.BinaryVersion
}

func fullCommand(cmd *cobra.Command) string {
	commands := []string{cmd.Use}
	for cmd.HasParent() {
		cmd = cmd.Parent()
		commands = append([]string{cmd.Use}, commands...)
	}
	return strings.Join(commands, " ")
}

func (b *Bundle) WriteInvocationInfo(ctx context.Context, cmd *cobra.Command, args []string) error {
	var flags []string
	cmd.Flags().Visit(func(pflag *pflag.Flag) {
		flags = append(flags, fmt.Sprintf("--%s %s", pflag.Name, pflag.Value.String()))
	})
	var fncmd []string
	fncmd = append(fncmd, filepath.Base(fullCommand(cmd)))
	fncmd = append(fncmd, flags...)
	fncmd = append(fncmd, args...)

	info := &InvocationInfo{
		Command: strings.Join(fncmd, " "),
		Os:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		NumCpu:  runtime.NumCPU(),
	}
	encodedInfo, err := json.Marshal(info)
	if err != nil {
		return fnerrors.InternalError("failed to marshal `InvocationInfo` as JSON: %w", err)
	}
	if err := b.WriteFile(ctx, "invocation_info.json", encodedInfo, 0600); err != nil {
		return fnerrors.InternalError("failed to write `InvocationInfo` to `invocation_info.json`: %w", err)
	}
	return nil
}

func (b *Bundle) WriteDockerInfo(ctx context.Context, dockerInfo *dockertypes.Info) error {
	if dockerInfo == nil {
		return nil
	}
	encodedInfo, err := json.Marshal(dockerInfo)
	if err != nil {
		return fnerrors.InternalError("failed to marshal docker `types.Info` as JSON: %w", err)
	}
	if err := b.WriteFile(ctx, "docker_info.json", encodedInfo, 0600); err != nil {
		return fnerrors.InternalError("failed to write docker `types.Info` to `docker_info.json`: %w", err)
	}
	return nil
}

func (b *Bundle) WriteMemStats(ctx context.Context) error {
	var mstats runtime.MemStats
	runtime.ReadMemStats(&mstats)

	encmstats, err := json.Marshal(mstats)
	if err != nil {
		return fnerrors.InternalError("failed to marshal `runtime.MemStats` as JSON: %w", err)
	}
	if err := b.WriteFile(ctx, "memstats.json", encmstats, 0600); err != nil {
		return fnerrors.InternalError("failed to write `runtime.MemStats` to `memstats.json`: %w", err)
	}
	return nil
}

func (b *Bundle) WriteErrorWithStacktrace(ctx context.Context, err error) error {
	errWithStack := fnerrors.MakeErrorStacktrace(err)

	encerr, err := json.Marshal(errWithStack)
	if err != nil {
		return fnerrors.InternalError("failed to marshal `ErrorWithStacktrace` as JSON: %w", err)
	}
	if err := b.WriteFile(ctx, "error.json", encerr, 0600); err != nil {
		return fnerrors.InternalError("failed to write `ErrorWithStacktrace` to `error.json`: %w", err)
	}
	return nil
}

// Guards access to file writes in the bundle.
func (b *Bundle) WriteFile(ctx context.Context, path string, contents []byte, mode fs.FileMode) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return fnfs.WriteFile(ctx, b.fsys, path, contents, mode)
}

func (b *Bundle) ReadInvocationInfo(ctx context.Context) (*InvocationInfo, error) {
	f, err := b.fsys.Open("invocation_info.json")
	if err != nil {
		return nil, fnerrors.InternalError("failed to open `invocation_info.json`: %w", err)
	}
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fnerrors.InternalError("failed to read `invocation_info.json`: %w", err)
	}
	var info InvocationInfo
	if err := json.Unmarshal(bytes, &info); err != nil {
		return nil, fnerrors.InternalError("failed to unmarshal `invocation_info.json`: %w", err)
	}
	return &info, nil
}

func (b *Bundle) EncryptTo(ctx context.Context, dst io.Writer) error {
	recipient, err := age.ParseX25519Recipient(publicKey)
	if err != nil {
		return fnerrors.BadInputError("failed to parse public key: %w", err)
	}
	gz := gzip.NewWriter(dst)
	defer gz.Close()

	encryptedWriter, _ := age.Encrypt(gz, recipient)
	defer encryptedWriter.Close()

	if err := maketarfs.TarFS(ctx, encryptedWriter, b.fsys, nil, nil); err != nil {
		return fnerrors.InternalError("failed to create encrypted bundle: %w", err)
	}
	return nil
}
