package output

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTableRendersRows(t *testing.T) {
	var b bytes.Buffer
	Table(&b, []string{"repo", "name"}, [][]string{{"sampleapp", "billing"}})
	out := b.String()
	if !strings.Contains(out, "repo") || !strings.Contains(out, "sampleapp") {
		t.Fatalf("unexpected table:\n%s", out)
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	if got := RelativeTime(now.Add(-2*time.Minute), now); got != "2m ago" {
		t.Fatalf("RelativeTime = %q", got)
	}
}
