package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
)

var (
	source = flag.String("source", "", "Source file.")
	target = flag.String("target", "", "Target file.")
)

func main() {
	flag.Parse()

	if *source == "" || *target == "" {
		log.Fatal("-source and -target are required")
	}

	if err := (func(ctx context.Context) error {
		mount := "/work"
		if err := os.MkdirAll(mount, 0777); err != nil {
			return err
		}

		io := rtypes.IO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}

		if err := runCommand(ctx, io, "mkfs.ext4",
			// Most images we create are small, but then can be extended. These base
			// images are created with the same parameters as a larger image would, so
			// we can get by resize2fsing them later.
			// Block size: 4k
			"-b", "4096",
			// Inode size: 256
			"-I", "256",
			// Don't defer work to first mount, do it now.
			"-E", "lazy_itable_init=0,lazy_journal_init=0",
			*target,
		); err != nil {
			return err
		}

		if err := runRawCommand(ctx, io, "mount", "-o", "loop", *target, mount); err != nil {
			return err
		}

		tarErr := runRawCommand(ctx, io, "tar", "xf", *source, "-C", mount)
		umountErr := runRawCommand(ctx, io, "umount", mount)
		fsckErr := runCommand(ctx, io, "e2fsck", "-y", "-f", *target)

		return multierr.New(tarErr, umountErr, fsckErr)
	})(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func runCommand(ctx context.Context, io rtypes.IO, command string, args ...string) error {
	loc, err := exec.LookPath(command)
	if err != nil {
		return err
	}

	return runRawCommand(ctx, io, loc, args...)
}

func runRawCommand(ctx context.Context, io rtypes.IO, command string, args ...string) error {
	fmt.Fprintf(os.Stderr, "Running: %s %s\n", command, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = io.Stdin
	cmd.Stdout = io.Stdout
	cmd.Stderr = io.Stderr
	return cmd.Run()
}
