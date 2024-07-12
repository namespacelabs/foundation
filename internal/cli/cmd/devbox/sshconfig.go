// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func newSetupSshAgentForwardingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup-ssh-agent-forwarding",
		Short: "Ensure that SSH agent forwarding is configured for Namespace Devboxes in your SSH config.",
	}

	sshConfigPath := cmd.Flags().String("ssh_config_path", "", "Path to your SSH config file. If not given, <home directory>.ssh/config is used.")
	disable := cmd.Flags().Bool("revert", false, "Revert to a state where the SSH agent forwarding is disabled for Namespace Devboxes.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return setupSshAgentForwarding(ctx, *sshConfigPath, *disable)
	})

	return cmd
}

func setupSshAgentForwarding(ctx context.Context, sshConfigPath string, disable bool) error {
	if sshConfigPath == "" {
		var err error
		sshConfigPath, err = getDefaultSshConfigPath()
		if err != nil {
			return err
		}
	}

	existingSshConfigState, err := checkExistingSshConfig(sshConfigPath)
	if err != nil {
		return fmt.Errorf("could not check ssh config '%s': %v", sshConfigPath, err)
	}
	switch existingSshConfigState {
	case sshConfigFileMissing:
		if disable {
			fmt.Fprintf(console.Stdout(ctx), "Nothing to do - '%s' does not exist so can not disable SSH agent forwarding on it.\n", sshConfigPath)
			return nil
		}
		return createSshConfig(ctx, sshConfigPath)
	case sshConfigNoAgentForwarding:
		if disable {
			fmt.Fprintf(console.Stdout(ctx), "SSH config '%s' has no SSH agent forwarding for Namespace Devboxes\n", sshConfigPath)
			return nil
		}
		return modifySshConfigEnableAgentForward(ctx, sshConfigPath)
	case sshConfigAgentForwarding:
		if !disable {
			fmt.Fprintf(console.Stdout(ctx), "SSH config '%s' already has SSH agent forwarding for Namespace Devboxes\n", sshConfigPath)
			return nil
		}
		return modifySshConfigDisableAgentForward(ctx, sshConfigPath)
	case sshConfigSectionModified:
		return fmt.Errorf("the SSH config section about devboxes has been modified - not touching SSH config '%s'", sshConfigPath)
	default:
		return fmt.Errorf("internal error: unexpected check result %d", existingSshConfigState)
	}
}

func confirmProceedFileModification(ctx context.Context, title string, printModInfo func(io.Writer) error) error {
	stdout := console.Stdout(ctx)
	fmt.Fprintf(stdout, "%s\n", lipgloss.NewStyle().Bold(true).Render(title))

	indented := indent(stdout)
	err := printModInfo(indented)
	if err != nil {
		return err
	}

	answer, err := tui.Ask(ctx, "Proceed?", "Type 'yes' to continue.", "")
	if err != nil {
		return err
	}

	if answer != "yes" {
		return fmt.Errorf("aborted on user request")
	}

	return nil
}

func createSshConfig(ctx context.Context, sshConfigPath string) error {
	title := fmt.Sprintf("Will create '%s' with the following contents:", sshConfigPath)
	err := confirmProceedFileModification(ctx, title, func(writer io.Writer) error {
		return writeTo(writer, getTemplateConfigLines())
	})
	if err != nil {
		return err
	}

	file, err := os.OpenFile(sshConfigPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	err = writeTo(file, getTemplateConfigLines())

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			parentDir := filepath.Dir(sshConfigPath)
			_, err := os.Stat(parentDir)
			if err != nil && errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("Could not write '%s' - try creating '%s' first", sshConfigPath, parentDir)
			}
		}
		return fmt.Errorf("Could not write '%s': %v", sshConfigPath, err)
	}

	fmt.Fprintf(console.Stdout(ctx), "Successfully wrote '%s\n", sshConfigPath)
	fmt.Fprintf(console.Stdout(ctx), "To remove the added config, re-run the command with --revert\n")

	return nil
}

func modifySshConfigEnableAgentForward(ctx context.Context, sshConfigPath string) error {
	// Simply append the config to the end
	title := fmt.Sprintf("Will append the following section to '%s':", sshConfigPath)
	err := confirmProceedFileModification(ctx, title, func(writer io.Writer) error {
		return writeTo(writer, getTemplateConfigLines())
	})
	if err != nil {
		return err
	}

	file, err := os.OpenFile(sshConfigPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("Could not open '%s': %v", sshConfigPath, err)
	}
	defer file.Close()

	writeTo(file, getTemplateConfigLines())

	fmt.Fprintf(console.Stdout(ctx), "Successfully appended to '%s\n", sshConfigPath)
	return nil
}

func modifySshConfigDisableAgentForward(ctx context.Context, sshConfigPath string) error {
	tempFile, err := copyToTempFileWithoutNamespaceDevboxSection(sshConfigPath)
	if tempFile != "" {
		defer os.Remove(tempFile)
	}
	if err != nil {
		return err
	}

	tempFileWithDiff, err := createDiff(ctx, sshConfigPath, tempFile)
	if tempFileWithDiff != "" {
		defer os.Remove(tempFileWithDiff)
	}
	if err != nil {
		return err
	}

	title := fmt.Sprintf("Will apply the following patch on '%s':", sshConfigPath)
	err = confirmProceedFileModification(ctx, title, func(writer io.Writer) error {
		f, err := os.Open(tempFileWithDiff)
		if err != nil {
			return fmt.Errorf("Failed to read diff from '%s': %v", tempFileWithDiff, err)
		}
		defer f.Close()

		_, err = io.Copy(writer, f)

		return err
	})
	if err != nil {
		return err
	}

	// Apply patch
	cmd := exec.CommandContext(ctx, "patch", "-u", "-b", sshConfigPath, tempFileWithDiff)
	output, err := cmd.CombinedOutput()
	if err != nil {
		console.Stderr(ctx).Write(output)

		return fmt.Errorf("patch failed: %v", err)
	}

	return nil
}

func createDiff(ctx context.Context, file1 string, file2 string) (string, error) {
	// Need to scan for "our section" and delete it.
	// checkExistingSshConfig already checked that it exists, but the file could have changed in the meantime.
	tempFile, err := os.CreateTemp("", "ssh_config_temp")
	if err != nil {
		return "", fmt.Errorf("could not create temp file: %v", err)
	}

	tempFilePath := tempFile.Name()
	defer tempFile.Close()

	diffCmd := exec.CommandContext(ctx, "diff", "-u", file1, file2)
	diffCmd.Stdout = tempFile
	diffErr := diffCmd.Run()
	switch diffErr.(type) {
	case *exec.ExitError:
		// Diff returns a non-zero exit code if its input files are different, which they should be.
	default: //couldnt run diff
		return tempFilePath, fmt.Errorf("Failed to run diff -u \"%s\" \"%s\": %v", file1, file2, diffErr)
	}

	return tempFilePath, nil
}

func copyToTempFileWithoutNamespaceDevboxSection(sshConfigPath string) (string, error) {
	// Need to scan for "our section" and delete it.
	// checkExistingSshConfig already checked that it exists, but the file could have changed in the meantime.
	tempFile, err := os.CreateTemp("", "ssh_config_temp")
	if err != nil {
		return "", fmt.Errorf("could not create temp file: %v", err)
	}

	tempFilePath := tempFile.Name()
	defer tempFile.Close()

	tempFileWriter := bufio.NewWriter(tempFile)

	file, err := os.Open(sshConfigPath)
	if err != nil {
		return tempFilePath, fmt.Errorf("could not open '%s': %v", sshConfigPath, err)
	}
	defer file.Close()

	templateLines := getTemplateConfigLines()
	lineInTemplate := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if lineInTemplate != len(templateLines) {
			if templateLines[lineInTemplate] == line {
				lineInTemplate++
				continue
			} else {
				return tempFilePath, fmt.Errorf("Namespace Devbox section in '%s' has been modified", sshConfigPath)
			}
		}

		if _, err := tempFileWriter.WriteString(line); err != nil {
			return tempFilePath, err
		}
		if _, err := tempFileWriter.WriteString("\n"); err != nil {
			return tempFilePath, err
		}
	}
	file.Close()

	if err := tempFileWriter.Flush(); err != nil {
		return tempFilePath, err
	}

	return tempFilePath, nil
}

type checkSshConfigResult int

const (
	sshConfigUnknown checkSshConfigResult = iota
	sshConfigFileMissing
	sshConfigNoAgentForwarding
	sshConfigAgentForwarding
	sshConfigSectionModified
)

func checkExistingSshConfig(sshConfigPath string) (checkSshConfigResult, error) {
	file, err := os.Open(sshConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sshConfigFileMissing, nil
		}
		return sshConfigUnknown, err
	}
	defer file.Close()

	templateLines := getTemplateConfigLines()
	lineInTemplate := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if lineInTemplate == len(templateLines) {
			// Already matched exactly. Only empty lines are accepted now, or a new directive (Host, Match)
			if isNewSection(line) {
				// The section around Namespace Devboxes matched exactly.
				return sshConfigAgentForwarding, nil
			}
			if !isEffectivelyEmpty(line) {
				// Relevant directives were added in the section around Namespace Devboxes.
				return sshConfigSectionModified, nil
			}
			continue
		}

		if templateLines[lineInTemplate] == line {
			lineInTemplate++
		} else {
			if lineInTemplate != 0 {
				// The beginning matched, but then there was something else.
				return sshConfigSectionModified, nil
			}
		}
	}

	if lineInTemplate == len(templateLines) {
		// The section around Namespace Devboxes matched exactly and there was nothing after it.
		return sshConfigAgentForwarding, nil
	}

	if err := scanner.Err(); err != nil {
		return sshConfigUnknown, err
	}

	return sshConfigNoAgentForwarding, nil
}

var (
	// Only whitespace, maybe something after a # (comment)
	effectivelyEmptyRegexp = regexp.MustCompile("^\\s*(#.*|)$")
	newSectionRegexp       = regexp.MustCompile("^\\s*(Host|Match)\\s.+")
)

func isEffectivelyEmpty(line string) bool {
	return effectivelyEmptyRegexp.MatchString(line)
}

func isNewSection(line string) bool {
	return newSectionRegexp.MatchString(line)
}

const DEVBOX_SSH_HOST_PATTERN = "ssh.*.namespace.so"

func getTemplateConfigLines() []string {
	return []string{
		"Host " + DEVBOX_SSH_HOST_PATTERN,
		"    # Allow SSH agent forwarding for Namespace Devboxes",
		"    ForwardAgent yes",
		"",
	}
}

func writeTo(writer io.Writer, lines []string) error {
	buffer := bufio.NewWriter(writer)

	for _, line := range lines {
		// It is OK to ignore the length because bufio.Writer guarantees that it returns an error if not all bytes were written.
		_, err := buffer.WriteString(line)
		if err != nil {
			return err
		}
		_, err = buffer.WriteString("\n")
		if err != nil {
			return err
		}
	}

	return buffer.Flush()
}

func offerSetupSshAgentForwarding(ctx context.Context) error {
	userVisible, err := offerSetupSshAgentForwardingInner(ctx)
	if err != nil {
		if userVisible {
			return err
		}
		fmt.Fprintf(console.Debug(ctx), "failed in offerSetupSshForwardingInner: %v", err)
	}
	return nil
}

const (
	offerAnswerNo = iota
	offerAnswerNoAndDontAsk
	offerAnswerYes
)

func offerSetupSshAgentForwardingInner(ctx context.Context) (bool, error) {
	configDir, err := dirs.Ensure(dirs.Config())
	if err != nil {
		return false, err
	}

	dontAskFlagFile := filepath.Join(configDir, "dont_ask_devbox_setup_ssh_agent_forwarding")
	if _, err := os.Stat(dontAskFlagFile); !errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	sshConfigPath, err := getDefaultSshConfigPath()
	if err != nil {
		return false, err
	}

	existingSshConfigState, err := checkExistingSshConfig(sshConfigPath)
	if err != nil {
		return false, err
	}

	if existingSshConfigState != sshConfigFileMissing && existingSshConfigState != sshConfigNoAgentForwarding {
		return false, nil
	}

	fmt.Fprintf(console.Stdout(ctx), "nsc has detected that SSH Agent forwarding is not configured for Namespace Devboxes.\n")
	fmt.Fprintf(console.Stdout(ctx), "Agent forwarding can be used to forward ssh agent based public key authentication to the devbox.\n")
	fmt.Fprintf(console.Stdout(ctx), "\n")

	options := []askOption{
		{value: offerAnswerNo, name: "No", description: "Don't add a section to my SSH config."},
		{value: offerAnswerNoAndDontAsk, name: "No and don't ask again", description: "Don't add a section to my SSH config and do not ask again."},
		{value: offerAnswerYes, name: "Yes", description: "Add a section to my SSH config."},
	}

	answer, err := askQuestion(ctx, options)
	if err != nil {
		return true, err
	}
	fmt.Fprintf(console.Stdout(ctx), "\n")

	if answer == offerAnswerNo {
		return true, nil
	}
	if answer == offerAnswerNoAndDontAsk {
		// Persist a flag.
		file, err := os.OpenFile(dontAskFlagFile, os.O_RDONLY|os.O_CREATE, 0644)
		if err != nil {
			return true, err
		}
		file.Close()

		fmt.Fprintf(console.Stdout(ctx), "OK! If you change your mind, you can run `nsc devbox setup-ssh-agent-forwarding`\n.")

		return true, nil
	}

	err = setupSshAgentForwarding(ctx, sshConfigPath, false)
	return true, err
}

func getDefaultSshConfigPath() (string, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home dir: %v", err)
	}
	return filepath.Join(userHome, ".ssh", "config"), nil
}

func askQuestion(ctx context.Context, options []askOption) (int, error) {
	item, err := tui.ListSelect(ctx, "Setup SSH agent forwarding for "+DEVBOX_SSH_HOST_PATTERN+" now?", options)
	if err != nil {
		return 0, err
	}

	if item == nil {
		return 0, context.Canceled
	}

	return item.(askOption).value, nil
}

type askOption struct {
	name        string
	description string
	value       int
}

func (d askOption) Title() string       { return d.name }
func (d askOption) Description() string { return d.description }
func (d askOption) FilterValue() string { return d.name }
