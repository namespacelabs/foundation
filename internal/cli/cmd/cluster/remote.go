// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewInstanceDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download [instance-id] <remote-path> [local-path]",
		Short: "Download a file from an instance.",
		Args:  cobra.RangeArgs(1, 3),
	}

	containerName := cmd.Flags().StringP("container_name", "c", "", "Target a container by name.")
	mkdir := cmd.Flags().Bool("mkdir", false, "Create parent directories locally if they don't exist.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, args, err := SelectRunningCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusterList) {
				PrintCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		if len(args) < 1 {
			return fnerrors.BadInputError("remote path is required")
		}

		remotePath := args[0]
		localPath := ""
		if len(args) >= 2 {
			localPath = args[1]
		}

		return instanceDownload(ctx, cluster, remotePath, localPath, *containerName, *mkdir)
	})

	return cmd
}

func NewInstanceUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload [instance-id] <local-path> <remote-path>",
		Short: "Upload a file to an instance.",
		Args:  cobra.RangeArgs(2, 3),
	}

	containerName := cmd.Flags().StringP("container_name", "c", "", "Target a container by name.")
	mkdir := cmd.Flags().Bool("mkdir", false, "Create parent directories on the remote if they don't exist.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, args, err := SelectRunningCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusterList) {
				PrintCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		if len(args) < 2 {
			return fnerrors.BadInputError("local path and remote path are required")
		}

		localPath := args[0]
		remotePath := args[1]

		return instanceUpload(ctx, cluster, localPath, remotePath, *mkdir, *containerName)
	})

	return cmd
}

func instanceDownload(ctx context.Context, cluster *api.KubernetesCluster, remotePath, localPath, containerName string, mkdir bool) error {
	connect := makeSSHConnect(cluster)

	return withSsh(ctx, connect, "root", func(ctx context.Context, client *ssh.Client) error {
		if containerName != "" {
			root, err := resolveContainerRoot(client, containerName)
			if err != nil {
				return err
			}
			remotePath = filepath.Join(root, remotePath)
		}

		session, err := client.NewSession()
		if err != nil {
			return err
		}
		defer session.Close()

		stdout, err := session.StdoutPipe()
		if err != nil {
			return err
		}

		if err := session.Start(fmt.Sprintf("cat %q", remotePath)); err != nil {
			return fnerrors.Newf("failed to start remote read: %w", err)
		}

		var dest io.Writer
		var tempFile string
		var f *os.File

		switch localPath {
		case "-":
			dest = os.Stdout
		case "":
			tmp, err := os.CreateTemp("", "nsc-download-*")
			if err != nil {
				return fnerrors.Newf("failed to create temp file: %w", err)
			}
			f = tmp
			dest = tmp
			tempFile = tmp.Name()
		default:
			if mkdir {
				localDir := filepath.Dir(localPath)
				if err := os.MkdirAll(localDir, 0755); err != nil {
					return fnerrors.Newf("failed to create local directory %q: %w", localDir, err)
				}
			}

			file, err := os.Create(localPath)
			if err != nil {
				return fnerrors.Newf("failed to create local file %q: %w", localPath, err)
			}
			f = file
			dest = file
		}

		if f != nil {
			defer f.Close()
		}

		if _, err := io.Copy(dest, stdout); err != nil {
			return fnerrors.Newf("failed to write output: %w", err)
		}

		if err := session.Wait(); err != nil {
			if localPath != "-" && localPath != "" {
				os.Remove(localPath)
			} else if tempFile != "" {
				os.Remove(tempFile)
			}
			return fnerrors.Newf("remote read failed: %w", err)
		}

		if tempFile != "" {
			fmt.Fprintln(os.Stderr, tempFile)
		}

		return nil
	})
}

func instanceUpload(ctx context.Context, cluster *api.KubernetesCluster, localPath, remotePath string, mkdir bool, containerName string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fnerrors.Newf("failed to open local file %q: %w", localPath, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fnerrors.Newf("failed to stat local file %q: %w", localPath, err)
	}

	connect := makeSSHConnect(cluster)

	return withSsh(ctx, connect, "root", func(ctx context.Context, client *ssh.Client) error {
		if containerName != "" {
			root, err := resolveContainerRoot(client, containerName)
			if err != nil {
				return err
			}
			remotePath = filepath.Join(root, remotePath)
		}

		if mkdir {
			session, err := client.NewSession()
			if err != nil {
				return err
			}

			remoteDir := filepath.Dir(remotePath)
			if err := session.Run(fmt.Sprintf("mkdir -p %q", remoteDir)); err != nil {
				session.Close()
				return fnerrors.Newf("failed to create remote directory %q: %w", remoteDir, err)
			}
			session.Close()
		}

		session, err := client.NewSession()
		if err != nil {
			return err
		}
		defer session.Close()

		stdin, err := session.StdinPipe()
		if err != nil {
			return err
		}

		if err := session.Start(fmt.Sprintf("cat > %q", remotePath)); err != nil {
			return fnerrors.Newf("failed to start remote write: %w", err)
		}

		n, err := io.Copy(stdin, f)
		if err != nil {
			return fnerrors.Newf("failed to copy file content: %w", err)
		}

		if n != stat.Size() {
			return fnerrors.Newf("incomplete write: wrote %d bytes, expected %d", n, stat.Size())
		}

		if err := stdin.Close(); err != nil {
			return fnerrors.Newf("failed to close stdin: %w", err)
		}

		if err := session.Wait(); err != nil {
			return fnerrors.Newf("remote write failed: %w", err)
		}

		return nil
	})
}

func makeSSHConnect(cluster *api.KubernetesCluster) ConnectSshFunc {
	return func(ctx context.Context, user string) (ConnectBits, error) {
		sshSvc := api.ClusterService(cluster, "ssh")
		if sshSvc == nil || sshSvc.Endpoint == "" {
			return ConnectBits{}, fnerrors.Newf("instance does not have ssh")
		}

		if sshSvc.Status != "READY" {
			return ConnectBits{}, fnerrors.Newf("expected ssh to be READY, saw %q", sshSvc.Status)
		}

		signer, err := ssh.ParsePrivateKey(cluster.SshPrivateKey)
		if err != nil {
			return ConnectBits{}, err
		}

		peerConn, err := api.DialEndpoint(ctx, sshSvc.Endpoint)
		if err != nil {
			return ConnectBits{}, err
		}

		if user == "" {
			user = "root"
		}

		return ConnectBits{Conn: peerConn, Signer: signer, Username: user}, nil
	}
}

func resolveContainerRoot(client *ssh.Client, containerName string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var stdout bytes.Buffer
	session.Stdout = &stdout

	if err := session.Run(fmt.Sprintf("nerdctl container inspect --format '{{.State.Pid}}' %q", containerName)); err != nil {
		return "", fnerrors.Newf("failed to inspect container %q: %w", containerName, err)
	}

	pid := strings.TrimSpace(stdout.String())
	if pid == "" || pid == "0" {
		return "", fnerrors.Newf("container %q is not running", containerName)
	}

	return fmt.Sprintf("/proc/%s/root", pid), nil
}
