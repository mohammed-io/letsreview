package gitdiff

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Mode string

const (
	ModeWorking Mode = "working"
	ModeStaged  Mode = "staged"
	ModeRefs    Mode = "refs"
)

type Request struct {
	RepoPath     string `json:"repoPath"`
	Mode         Mode   `json:"mode"`
	BaseRef      string `json:"baseRef"`
	HeadRef      string `json:"headRef"`
	ContextLines int    `json:"contextLines,omitempty"`
}

type File struct {
	Path      string `json:"path"`
	OldPath   string `json:"oldPath,omitempty"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Hunks     []Hunk `json:"hunks"`
}

type Hunk struct {
	Header string `json:"header"`
	Lines  []Line `json:"lines"`
}

type Line struct {
	Kind      string `json:"kind"`
	OldNumber int    `json:"oldNumber,omitempty"`
	NewNumber int    `json:"newNumber,omitempty"`
	Text      string `json:"text"`
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

func Load(ctx context.Context, req Request) ([]File, error) {
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo path: %w", err)
	}
	if err := ensureGitRepo(ctx, repo); err != nil {
		return nil, err
	}

	args, err := diffArgs(req)
	if err != nil {
		return nil, err
	}

	output, err := runGit(ctx, repo, args...)
	if err != nil {
		return nil, err
	}
	return ParseUnified(output), nil
}

func Summary(files []File) string {
	if len(files) == 0 {
		return "No diff lines found for this comparison."
	}

	additions, deletions := 0, 0
	kinds := map[string]int{}
	names := make([]string, 0, len(files))
	for _, file := range files {
		additions += file.Additions
		deletions += file.Deletions
		kinds[file.Status]++
		names = append(names, file.Path)
	}

	return fmt.Sprintf("%d files changed with %d additions and %d deletions. Main files: %s.", len(files), additions, deletions, strings.Join(names, ", "))
}

func ExplainSelection(files []File, filePath string, startLine int, endLine int) string {
	lines := selectedLines(files, filePath, startLine, endLine)
	if len(lines) == 0 {
		return "No selected diff lines were found."
	}

	added, removed, context := 0, 0, 0
	for _, line := range lines {
		switch line.Kind {
		case "add":
			added++
		case "del":
			removed++
		default:
			context++
		}
	}

	return fmt.Sprintf("Selection includes %d added, %d removed, and %d context lines. It likely changes behavior around `%s`.", added, removed, context, compactSnippet(lines))
}

func ParseUnified(diff string) []File {
	lines := strings.Split(diff, "\n")
	files := []File{}
	var current *File
	var currentHunk *Hunk
	oldLine, newLine := 0, 0

	flushHunk := func() {
		if current != nil && currentHunk != nil {
			current.Hunks = append(current.Hunks, *currentHunk)
			currentHunk = nil
		}
	}
	flushFile := func() {
		if current != nil {
			flushHunk()
			files = append(files, *current)
			current = nil
		}
	}

	for _, raw := range lines {
		switch {
		case strings.HasPrefix(raw, "diff --git "):
			flushFile()
			current = &File{Status: "modified"}
			parts := strings.Fields(raw)
			if len(parts) >= 4 {
				current.OldPath = strings.TrimPrefix(parts[2], "a/")
				current.Path = strings.TrimPrefix(parts[3], "b/")
			}
		case current != nil && strings.HasPrefix(raw, "new file mode"):
			current.Status = "added"
		case current != nil && strings.HasPrefix(raw, "deleted file mode"):
			current.Status = "deleted"
		case current != nil && strings.HasPrefix(raw, "rename from "):
			current.Status = "renamed"
			current.OldPath = strings.TrimPrefix(raw, "rename from ")
		case current != nil && strings.HasPrefix(raw, "rename to "):
			current.Path = strings.TrimPrefix(raw, "rename to ")
		case current != nil && strings.HasPrefix(raw, "+++ "):
			path := strings.TrimPrefix(raw, "+++ ")
			if path != "/dev/null" {
				current.Path = strings.TrimPrefix(path, "b/")
			}
		case current != nil && strings.HasPrefix(raw, "@@ "):
			flushHunk()
			currentHunk = &Hunk{Header: raw}
			oldLine, newLine = parseHunkStart(raw)
		case current != nil && currentHunk != nil:
			if raw == `\ No newline at end of file` {
				continue
			}
			line := Line{Kind: "ctx", Text: raw}
			switch {
			case strings.HasPrefix(raw, "+"):
				line.Kind = "add"
				line.NewNumber = newLine
				line.Text = strings.TrimPrefix(raw, "+")
				newLine++
				current.Additions++
			case strings.HasPrefix(raw, "-"):
				line.Kind = "del"
				line.OldNumber = oldLine
				line.Text = strings.TrimPrefix(raw, "-")
				oldLine++
				current.Deletions++
			default:
				line.OldNumber = oldLine
				line.NewNumber = newLine
				line.Text = strings.TrimPrefix(raw, " ")
				oldLine++
				newLine++
			}
			currentHunk.Lines = append(currentHunk.Lines, line)
		}
	}
	flushFile()
	return files
}

func diffArgs(req Request) ([]string, error) {
	contextArg := fmt.Sprintf("--unified=%d", normalizedContextLines(req.ContextLines))
	switch req.Mode {
	case "", ModeWorking:
		return []string{"diff", "--no-ext-diff", contextArg, "HEAD", "--"}, nil
	case ModeStaged:
		return []string{"diff", "--no-ext-diff", "--cached", contextArg, "--"}, nil
	case ModeRefs:
		if strings.TrimSpace(req.BaseRef) == "" || strings.TrimSpace(req.HeadRef) == "" {
			return nil, errors.New("baseRef and headRef are required for refs mode")
		}
		return []string{"diff", "--no-ext-diff", contextArg, req.BaseRef, req.HeadRef, "--"}, nil
	default:
		return nil, fmt.Errorf("unsupported diff mode %q", req.Mode)
	}
}

func normalizedContextLines(value int) int {
	if value <= 0 {
		return 3
	}
	if value > 200 {
		return 200
	}
	return value
}

func ensureGitRepo(ctx context.Context, repo string) error {
	output, err := runGit(ctx, repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("%s is not a git repository: %w", repo, err)
	}
	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("%s is not a git repository", repo)
	}
	return nil
}

func runGit(parent context.Context, repo string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return string(output), nil
}

func parseHunkStart(header string) (int, int) {
	matches := hunkHeader.FindStringSubmatch(header)
	if len(matches) != 3 {
		return 0, 0
	}
	oldStart, _ := strconv.Atoi(matches[1])
	newStart, _ := strconv.Atoi(matches[2])
	return oldStart, newStart
}

func selectedLines(files []File, filePath string, startLine int, endLine int) []Line {
	if endLine < startLine {
		startLine, endLine = endLine, startLine
	}

	for _, file := range files {
		if file.Path != filePath {
			continue
		}
		selected := []Line{}
		flatIndex := 0
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				flatIndex++
				if flatIndex >= startLine && flatIndex <= endLine {
					selected = append(selected, line)
				}
			}
		}
		return selected
	}
	return nil
}

func compactSnippet(lines []Line) string {
	parts := []string{}
	for _, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text != "" {
			parts = append(parts, text)
		}
		if len(parts) == 3 {
			break
		}
	}
	return strings.Join(parts, " / ")
}
