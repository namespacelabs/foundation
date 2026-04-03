package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFetchLogsRequestTimestampRangeUsesRFC3339(t *testing.T) {
	after := time.Date(2026, time.April, 3, 3, 0, 34, 0, time.UTC)
	before := time.Date(2026, time.April, 3, 3, 0, 35, 0, time.UTC)

	req := FetchLogsRequest{
		TimestampRange: &TimestampRange{
			After:  &after,
			Before: &before,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	jsonBody := string(data)
	if !strings.Contains(jsonBody, `"after":"2026-04-03T03:00:34Z"`) {
		t.Fatalf("json.Marshal() = %s, want RFC3339 after timestamp", jsonBody)
	}

	if !strings.Contains(jsonBody, `"before":"2026-04-03T03:00:35Z"`) {
		t.Fatalf("json.Marshal() = %s, want RFC3339 before timestamp", jsonBody)
	}

	if strings.Contains(jsonBody, `"seconds"`) || strings.Contains(jsonBody, `"nanos"`) {
		t.Fatalf("json.Marshal() = %s, want protojson-style timestamp strings", jsonBody)
	}
}
