// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package host

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

const nixosDynsym = ".dynloader"

func EnsureNixosPatched(ctx context.Context, binary string) (string, error) {
	// We keep a symlink to the interpreter go/bin/go was patched with. If the link
	// exists, then the interpreter exists, and no additional patching is required.
	dynlink := binary + nixosDynsym
	patchedGoBin := binary + ".patched"

	if _, err := os.Readlink(dynlink); err == nil {
		return patchedGoBin, nil
	}

	return tasks.Return(ctx, tasks.Action("nixos.patch-elf").Arg("path", binary), func(ctx context.Context) (string, error) {
		// Assume all NixOS versions have systemd, which is true for all recent versions.
		systemCtl, err := output(ctx, "which", "systemctl")
		if err != nil {
			return "", err
		}

		existingInterpreter, err := output(ctx, "nix-shell", "-p", "patchelf", "--run", fmt.Sprintf("patchelf --print-interpreter %s", systemCtl))
		if err != nil {
			return "", err
		}

		cint := strings.TrimSpace(string(existingInterpreter))

		if _, err := output(ctx, "nix-shell", "-p", "patchelf", "--run", fmt.Sprintf("patchelf --set-interpreter %s --output %s %s", cint, patchedGoBin, binary)); err != nil {
			return "", err
		}

		if err := os.Remove(dynlink); err != nil && os.IsNotExist(err) {
			fmt.Fprintf(console.Errors(ctx), "Failed to remove %s: %v", dynlink, err)
		}

		// Remember which interpreter we used, and if it doesn't exist, re-patch.
		if err := os.Symlink(cint, dynlink); err != nil {
			return "", err
		}

		return patchedGoBin, nil
	})
}

func IsNixOS() bool {
	_, err := os.Stat("/etc/NIXOS")
	return err == nil
}

func output(ctx context.Context, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, args[0], args[1:]...)

	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = console.Stderr(ctx)
	c.Stdin = nil
	if err := c.Run(); err != nil {
		return nil, fnerrors.InternalError("failed to invoke: %w", err)
	}
	return b.Bytes(), nil
}
