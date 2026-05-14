package gitdiff

import (
	"strings"
	"testing"
)

func TestParseUnifiedReportsFilesHunksAndLineStats(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"index 1111111..2222222 100644",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,3 +1,4 @@",
		" package main",
		"-func old() {}",
		"+func newThing() {}",
		"+const ready = true",
		" func keep() {}",
		"",
	}, "\n")

	files := ParseUnified(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	file := files[0]
	if file.Path != "main.go" {
		t.Fatalf("expected path main.go, got %q", file.Path)
	}
	if file.Additions != 2 || file.Deletions != 1 {
		t.Fatalf("expected +2 -1, got +%d -%d", file.Additions, file.Deletions)
	}
	if len(file.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(file.Hunks))
	}
	if file.Hunks[0].Lines[1].Kind != "del" || file.Hunks[0].Lines[1].OldNumber != 2 {
		t.Fatalf("expected deleted old line 2, got %#v", file.Hunks[0].Lines[1])
	}
	if file.Hunks[0].Lines[2].Kind != "add" || file.Hunks[0].Lines[2].NewNumber != 2 {
		t.Fatalf("expected added new line 2, got %#v", file.Hunks[0].Lines[2])
	}
}

func TestExplainSelectionSummarizesObservableSelectedLines(t *testing.T) {
	files := []File{
		{
			Path: "main.go",
			Hunks: []Hunk{
				{Lines: []Line{
					{Kind: "ctx", Text: "func keep() {}"},
					{Kind: "del", Text: "return false"},
					{Kind: "add", Text: "return true"},
				}},
			},
		},
	}

	summary := ExplainSelection(files, "main.go", 2, 3)
	if !strings.Contains(summary, "1 added") || !strings.Contains(summary, "1 removed") {
		t.Fatalf("expected selected line counts in summary, got %q", summary)
	}
}
