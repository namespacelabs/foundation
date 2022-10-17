// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"io"

	"namespacelabs.dev/foundation/internal/console/common"
)

type ActionSink interface {
	Waiting(*RunningAction)
	Started(*RunningAction)
	Done(*RunningAction)
	Instant(*EventData)
	AttachmentsUpdated(ActionID, *ResultData)
	Output(name, contentType string, outputType common.CatOutputType) io.Writer
}

func NullSink() ActionSink {
	return &nullSink{}
}

type nullSink struct{}

func (nullSink) Waiting(*RunningAction)                               {}
func (nullSink) Started(*RunningAction)                               {}
func (nullSink) Done(*RunningAction)                                  {}
func (nullSink) Instant(*EventData)                                   {}
func (nullSink) AttachmentsUpdated(ActionID, *ResultData)             {}
func (nullSink) Output(_, _ string, _ common.CatOutputType) io.Writer { return nil }
