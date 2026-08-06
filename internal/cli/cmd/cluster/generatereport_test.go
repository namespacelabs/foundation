// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"reflect"
	"strconv"
	"testing"
)

func TestReportColumns(t *testing.T) {
	fullRecord := make([]string, len(baseReportHeader)+len(githubReportHeader)+len(buildkiteReportHeader))
	for i := range fullRecord {
		fullRecord[i] = strconv.Itoa(i)
	}
	baseEnd := len(baseReportHeader)
	githubEnd := baseEnd + len(githubReportHeader)

	for _, test := range []struct {
		name             string
		includeGithub    bool
		includeBuildkite bool
		wantHeader       []string
		wantRecord       []string
	}{
		{name: "base only", wantHeader: baseReportHeader, wantRecord: fullRecord[:baseEnd]},
		{name: "GitHub", includeGithub: true, wantHeader: append(append([]string{}, baseReportHeader...), githubReportHeader...), wantRecord: fullRecord[:githubEnd]},
		{name: "Buildkite", includeBuildkite: true, wantHeader: append(append([]string{}, baseReportHeader...), buildkiteReportHeader...), wantRecord: append(append([]string{}, fullRecord[:baseEnd]...), fullRecord[githubEnd:]...)},
		{name: "both", includeGithub: true, includeBuildkite: true, wantHeader: append(append(append([]string{}, baseReportHeader...), githubReportHeader...), buildkiteReportHeader...), wantRecord: fullRecord},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := reportHeader(test.includeGithub, test.includeBuildkite); !reflect.DeepEqual(got, test.wantHeader) {
				t.Errorf("reportHeader() = %v, want %v", got, test.wantHeader)
			}
			if got := selectReportColumns(fullRecord, test.includeGithub, test.includeBuildkite); !reflect.DeepEqual(got, test.wantRecord) {
				t.Errorf("selectReportColumns() = %v, want %v", got, test.wantRecord)
			}
		})
	}
}
