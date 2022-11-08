package filewatcher

import "testing"

func TestLongestCommonPathPrefix(t *testing.T) {
	list := []string{
		"/path/to/server/file",
		"/path/to/service",
		"/path/to/shared/",
	}
	got := longestCommonPathPrefix(list)
	want := "/path/to"
	if got != want {
		t.Errorf("longestCommonPathPrefix%v: got %q, want %q", list, got, want)
	}
}
