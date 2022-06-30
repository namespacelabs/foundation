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
	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	stacktraceserializer "namespacelabs.dev/foundation/internal/fnerrors/stacktrace/serializer"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/maketarfs"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace/tasks/protocol"
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

func (b *Bundle) WriteError(ctx context.Context, err error) error {
	errstack, err := stacktraceserializer.NewErrorStacktrace(err)
	if err != nil {
		return err
	}
	encstack, err := json.Marshal(errstack)
	if err != nil {
		return fnerrors.InternalError("failed to marshal `ErrorStacktrace` as JSON: %w", err)
	}
	if err := b.WriteFile(ctx, "error.json", encstack, 0600); err != nil {
		return fnerrors.InternalError("failed to write `ErrorStacktrace` to `error.json`: %w", err)
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

func (b *Bundle) unmarshalStoredTaskFromActionLogs(path string) (*protocol.StoredTask, error) {
	f, err := b.fsys.Open(path)
	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	storedTask := &protocol.StoredTask{}

	if err := (prototext.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}).Unmarshal(content, storedTask); err != nil {
		return nil, err
	} else {
		return storedTask, nil
	}
}

func (b *Bundle) unmarshalAttachment(path string) (*storage.Command_Log, error) {
	f, err := b.fsys.Open(path)
	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return &storage.Command_Log{
		Id:      path,
		Content: content,
	}, nil
}

func (b *Bundle) ActionLogs(ctx context.Context, debug io.Writer) (*storage.Command, error) {
	cmd := &storage.Command{}

	var errs []error

	err := fs.WalkDir(b.fsys, ".", func(path string, dirent fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(debug, "Failed to walk %s: %v\n", path, err)
			return nil
		}

		if path == "." {
			return nil
		}

		// Unmarshal the action log file if present.
		if strings.HasSuffix(path, "action.textpb") {
			if storedTask, err := b.unmarshalStoredTaskFromActionLogs(path); err != nil {
				fmt.Fprintf(debug, "Failed to unmarshal stored task from action logs for path %s: %v\n", path, err)
				errs = append(errs, err)
			} else {
				fmt.Fprintf(debug, "Writing stored task %s from path %s\n", storedTask.Name, path)
				cmd.ActionLog = append(cmd.ActionLog, storedTask)
			}
			return nil
		} else if attachment, err := b.unmarshalAttachment(path); err == nil {
			// Unmarshal the attachment file if present. Unlike the action log, we only log if we
			// fail retrieving attachments.
			fmt.Fprintf(debug, "Writing stored attachment with ID %q from path %s\n", attachment.Id, path)
			cmd.AttachedLog = append(cmd.AttachedLog, attachment)
		} else {
			fmt.Fprintf(debug, "Failed to unmarshal artifact from path %s: %v\n", path, err)
		}

		return nil
	})

	errs = append(errs, err)

	return cmd, multierr.New(errs...)
}

func (b *Bundle) EncryptTo(ctx context.Context, dst io.Writer) error {
	recipient, err := age.ParseX25519Recipient(publicKey)
	if err != nil {
		return fnerrors.BadInputError("failed to parse public key: %w", err)
	}

	encWriter, err := age.Encrypt(dst, recipient)
	if err != nil {
		return fnerrors.InternalError("failed to encrypt bundle: %w", err)
	}
	defer encWriter.Close()

	gzWriter := gzip.NewWriter(encWriter)
	defer gzWriter.Close()

	if err := maketarfs.TarFS(ctx, gzWriter, b.fsys, nil, nil); err != nil {
		return fnerrors.InternalError("failed to archive bundle: %w", err)
	}

	return nil
}
