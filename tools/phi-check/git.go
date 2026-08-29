package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// changedFiles returns files that differ from base (added/copied/modified/
// renamed), filtered to scannable extensions. It prefers the merge-base so a
// stale base branch doesn't flag unrelated files.
func changedFiles(base string) ([]string, error) {
	ref := base
	if mb, err := run("git", "merge-base", base, "HEAD"); err == nil {
		if s := strings.TrimSpace(mb); s != "" {
			ref = s
		}
	}

	out, err := run("git", "diff", "--name-only", "--diff-filter=ACMR", ref+"...HEAD")
	if err != nil {
		return nil, fmt.Errorf("git diff failed (is %q a valid ref?): %w", base, err)
	}

	var files []string
	for _, line := range strings.Split(out, "\n") {
		p := strings.TrimSpace(line)
		if p == "" {
			continue
		}
		if scannableExts[strings.ToLower(filepath.Ext(p))] {
			files = append(files, p)
		}
	}
	return files, nil
}

func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	b, err := cmd.Output()
	return string(b), err
}
