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
)

const (
	// Public key used to encrypt bundles before uploading to the bundle service.
	// Generated with `age-keygen` and needs to be kept in sync with the private
	// internal key available only to foundation core devs.
	publicKey = "age1ngp9m4wrhq4zvc2redr7jm8gat0qnkue4dfsklqdxg5yn7w0xsqqwp3jgw"

	InvocationInfoFile = "invocation_info.json"
	DockerInfoFile     = "docker_info.json"
	MemstatsFile       = "memstats.json"
	ErrorFile          = "error.json"
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

func NewBundle(fsys fnfs.ReadWriteFS, timestamp time.Time) *Bundle {
	return &Bundle{
		fsys:      fsys,
		Timestamp: timestamp,
	}
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
	if err := b.WriteFile(ctx, InvocationInfoFile, encodedInfo, 0600); err != nil {
		return fnerrors.InternalError("failed to write `InvocationInfo` to %q: %w", InvocationInfoFile, err)
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
	if err := b.WriteFile(ctx, DockerInfoFile, encodedInfo, 0600); err != nil {
		return fnerrors.InternalError("failed to write docker `types.Info` to %q: %w", DockerInfoFile, err)
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
	if err := b.WriteFile(ctx, MemstatsFile, encmstats, 0600); err != nil {
		return fnerrors.InternalError("failed to write `runtime.MemStats` to %q: %w", MemstatsFile, err)
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
	if err := b.WriteFile(ctx, ErrorFile, encstack, 0600); err != nil {
		return fnerrors.InternalError("failed to write `ErrorStacktrace` to %q: %w", ErrorFile, err)
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
	f, err := b.fsys.Open(InvocationInfoFile)
	if err != nil {
		return nil, fnerrors.InternalError("failed to open %q: %w", InvocationInfoFile, err)
	}
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fnerrors.InternalError("failed to read %q: %w", InvocationInfoFile, err)
	}
	var info InvocationInfo
	if err := json.Unmarshal(bytes, &info); err != nil {
		return nil, fnerrors.InternalError("failed to unmarshal %q: %w", InvocationInfoFile, err)
	}
	return &info, nil
}

func (b *Bundle) unmarshalStoredTaskFromActionLogs(path string) (*storage.StoredTask, error) {
	f, err := b.fsys.Open(path)
	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	storedTask := &storage.StoredTask{}

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

func (b *Bundle) ActionLogs(ctx context.Context) (*storage.Command, error) {
	cmd := &storage.Command{}

	var errs []error

	err := fs.WalkDir(b.fsys, ".", func(path string, dirent fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if path == "." {
			return nil
		}

		// Unmarshal the action log file if present.
		if strings.HasSuffix(path, "action.textpb") {
			if storedTask, err := b.unmarshalStoredTaskFromActionLogs(path); err != nil {
				errs = append(errs, err)
			} else {
				cmd.ActionLog = append(cmd.ActionLog, storedTask)
			}
			return nil
		} else {
			fileExtension := filepath.Ext(path)
			if fileExtension != "" {
				if attachment, err := b.unmarshalAttachment(path); err == nil {
					cmd.AttachedLog = append(cmd.AttachedLog, attachment)
				}
			}
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
