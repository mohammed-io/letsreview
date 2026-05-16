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

func TestSelectionStatsCountsLineKinds(t *testing.T) {
	files := []File{
		{
			Path: "main.go",
			Hunks: []Hunk{
				{Lines: []Line{
					{Kind: "ctx", NewNumber: 21, OldNumber: 21, Text: "func keep() {}"},
					{Kind: "del", OldNumber: 22, Text: "return false"},
					{Kind: "add", NewNumber: 22, Text: "return true"},
					{Kind: "add", NewNumber: 23, Text: "return ready"},
				}},
			},
		},
	}

	added, removed, context := SelectionStats(files, "main.go", 22, 23)
	if added != 2 || removed != 1 || context != 0 {
		t.Fatalf("expected 2 added, 1 removed, 0 context; got +%d -%d ctx%d", added, removed, context)
	}
}

func TestDiffArgsDefaultToGitHubLikeContext(t *testing.T) {
	args, err := diffArgs(Request{Mode: ModeWorking})
	if err != nil {
		t.Fatalf("diff args: %v", err)
	}
	if !contains(args, "--unified=3") {
		t.Fatalf("expected GitHub-like default context, got %#v", args)
	}
}

func TestDiffArgsHonorCustomContextWithLimit(t *testing.T) {
	args, err := diffArgs(Request{Mode: ModeStaged, ContextLines: 20})
	if err != nil {
		t.Fatalf("diff args: %v", err)
	}
	if !contains(args, "--unified=20") {
		t.Fatalf("expected custom context, got %#v", args)
	}

	args, err = diffArgs(Request{Mode: ModeWorking, ContextLines: 500})
	if err != nil {
		t.Fatalf("diff args: %v", err)
	}
	if !contains(args, "--unified=200") {
		t.Fatalf("expected capped context, got %#v", args)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
