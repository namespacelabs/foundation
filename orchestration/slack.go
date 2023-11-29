// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/schema"
)

func renderSlackMessage(plan *schema.DeployPlan, start, end time.Time, err error) []slack.Block {
	var blocks []slack.Block
	blocks = append(blocks, slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, timeEmoji(end, err)+" "+deployLabel(end), true, false)))
	blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType,
		fmt.Sprintf("%s *%s*%s with:",
			workingLabel(end, err),
			plan.GetEnvironment().GetName(),
			maybeFrom()), false, false), nil, nil))
	blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType,
		strings.Join(servers(plan), "\n"), false, false), nil, nil))
	if !end.IsZero() {
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType,
			maybeTook(start, end), false, false), nil, nil))
	}
	if err != nil {
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType,
			fmt.Sprintf("*Error* %v", err), false, false), nil, nil))
	}
	return blocks
}

func deployLabel(end time.Time) string {
	if end.IsZero() {
		return "Deploying"
	}

	return "Deployed"
}

func workingLabel(end time.Time, err error) string {
	if end.IsZero() {
		return "Updating"
	}

	if err != nil {
		return "Failed to update"
	}

	return "Updated"
}

func servers(plan *schema.DeployPlan) []string {
	var fo []string

	for _, ent := range plan.Stack.Entry {
		srv := ent.GetPackageName().String()
		if slices.Contains(plan.FocusServer, srv) {
			srv = fmt.Sprintf("*%s*", srv)
		}
		fo = append(fo, " · "+srv)
	}

	return fo
}

func timeEmoji(end time.Time, err error) string {
	if end.IsZero() {
		return ":hourglass_flowing_sand:"
	}

	if err != nil {
		return ":boom:"
	}

	return ":white_check_mark:"
}

func maybeTook(start, end time.Time) string {
	if end.IsZero() {
		return ""
	}

	return fmt.Sprintf("took %v", end.Sub(start))
}

func maybeFrom() string {
	if bkUrl := os.Getenv("BUILDKITE_BUILD_URL"); bkUrl != "" {
		name := os.Getenv("BUILDKITE_PIPELINE_NAME")
		if name == "" {
			name = "Buildkite"
		} else {
			if number := os.Getenv("BUILDKITE_BUILD_NUMBER"); number != "" {
				name += " #" + number
			}
		}

		if jobId := os.Getenv("BUILDKITE_JOB_ID"); jobId != "" {
			bkUrl += "#" + jobId
		}

		return fmt.Sprintf(" from <%s|%s>", bkUrl, name)
	}

	return ""
}
