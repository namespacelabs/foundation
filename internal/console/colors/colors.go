// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package colors

import (
	"context"

	"github.com/morikuni/aec"
)

type contextKey string

var (
	_styleKey = contextKey("fn.colors")
)

type Style struct {
	Header         Applicable // Timestamps, etc.
	Comment        Applicable // Faded out, at the end of a line.
	Progress       Applicable // Inline content that is updated w/ progress.
	ErrorHeader    Applicable // Error prefixes.
	LessRelevant   Applicable // A passing detail, like the fact that a list is empty.
	Highlight      Applicable // Something we should highlight, usually in bold.
	ErrorWhat      Applicable // When formatting errors, the kind of error.
	LogError       Applicable // An error displayed in a log line.
	LogCategory    Applicable // Log line category.
	LogCachedName  Applicable // Names, when cached.
	LogArgument    Applicable // An argument to an invocation.
	LogResult      Applicable // The result of an invocation.
	LogErrorReason Applicable // An expected error reason.
	LogScope       Applicable // Highlight of a package for which an invocation is being made.
	TestSuccess    Applicable // It says it on the tin.
	TestFailure    Applicable // Here too.
}

func WithStyle(ctx context.Context, s Style) context.Context {
	return context.WithValue(ctx, _styleKey, &s)
}

func Ctx(ctx context.Context) Style {
	if style := ctx.Value(_styleKey); style != nil {
		return *style.(*Style)
	}
	return NoColors
}

type Applicable interface {
	Apply(string) string
}

var WithColors = Style{
	Header:         aec.LightBlackF,
	Comment:        aec.LightBlackF,
	LogCategory:    aec.LightBlueF,
	LogCachedName:  aec.LightBlackF,
	Progress:       aec.LightBlackF,
	LogArgument:    aec.CyanF,
	LogResult:      aec.BlueF,
	LogErrorReason: aec.BlueF,
	LogError:       aec.RedF,
	LogScope:       aec.Italic,
	ErrorHeader:    aec.RedF.With(aec.Bold),
	LessRelevant:   aec.Italic,
	Highlight:      aec.Bold,
	ErrorWhat:      aec.MagentaF,
	TestSuccess:    aec.GreenF,
	TestFailure:    aec.RedF,
}

var NoColors = Style{
	Header:         noOpANSI,
	Comment:        noOpANSI,
	LogCategory:    noOpANSI,
	LogCachedName:  noOpANSI,
	Progress:       noOpANSI,
	LogArgument:    noOpANSI,
	LogResult:      noOpANSI,
	LogErrorReason: noOpANSI,
	LogError:       noOpANSI,
	LogScope:       noOpANSI,
	ErrorHeader:    noOpANSI,
	LessRelevant:   noOpANSI,
	Highlight:      noOpANSI,
	ErrorWhat:      noOpANSI,
	TestSuccess:    noOpANSI,
	TestFailure:    noOpANSI,
}

// An implementation of aec.ANSI that does completely nothing.
// It is more appropriate to use it in for non-TTY output since
// [aec.EmptyBuilder.ANSI] inserts reset codes "ESC[0m" regardless.
type noOpANSIImpl struct{}

func (noOpANSIImpl) Apply(s string) string {
	return s
}

var noOpANSI = noOpANSIImpl{}
