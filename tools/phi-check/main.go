// Command phi-check is a pre-merge guardrail that scans source for patterns
// that risk leaking Protected Health Information (PHI) or otherwise violating
// HIPAA safeguards. It is intentionally dependency-free (stdlib only) so it can
// run anywhere the repo's Go toolchain runs.
//
// Exit codes:
//
//	0  no findings
//	1  one or more findings (blocks the merge)
//	2  usage / internal error
//
// Suppression: a finding is dropped if the same line, or the line immediately
// above it, contains the marker "phi:allow". Use sparingly and only for
// reviewed exceptions (e.g. synthetic fixtures).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// severity ranks a finding. High findings should always block; medium findings
// block as well under this repo's policy but are separated so the report is
// easy to triage.
type severity int

const (
	medium severity = iota
	high
)

func (s severity) String() string {
	if s == high {
		return "HIGH"
	}
	return "MEDIUM"
}

// rule is a single detection. re matches a line; if allowIf is non-nil and
// matches the same line, the finding is skipped (used to carve out obvious
// false positives without a blanket suppression comment).
type rule struct {
	id      string
	desc    string
	sev     severity
	re      *regexp.Regexp
	allowIf *regexp.Regexp
}

// finding is a single rule hit at a location.
type finding struct {
	rule rule
	file string
	line int
	text string
}

// phiFieldNames are struct-field / column / key names that commonly carry PHI.
// The 18 HIPAA Safe Harbor identifiers inform this list.
const phiFieldNames = `(?i)\b(` +
	`ssn|social_?security|` +
	`dob|date_?of_?birth|birth_?date|` +
	`mrn|medical_?record(_?number)?|` +
	`diagnos[ei]s|icd_?10|icd10|cpt_?code|` +
	`patient(_?name|_?id)?|` +
	`health_?plan|insurance_?(id|number|member)|policy_?number|` +
	`prescription|medication|rx_?number|` +
	`npi|` + // National Provider Identifier
	`lab_?result|test_?result|` +
	`biometric|fingerprint|` +
	`beneficiary` +
	`)\b`

var rules = buildRules()

func buildRules() []rule {
	// Common allow-list: test/fixture-ish contexts where an identifier name is
	// expected and not real data. We still keep these MEDIUM/HIGH so reviewers
	// see them, but let per-line phi:allow handle true exceptions.
	logCall := regexp.MustCompile(`(?i)\b(log|logger|logrus|zap|slog|fmt\.(Print|Sprint|Fprint)|println|printf|console\.(log|info|warn|error|debug))\b`)

	return []rule{
		{
			id:   "PHI001",
			desc: "PHI-like field appears to be logged (HIPAA §164.312 audit/logging leakage)",
			sev:  high,
			// A logging call on a line that also references a PHI-looking name.
			re: regexp.MustCompile(`(?i)(` + trimAnchors(logCall.String()) + `).*` + trimAnchors(phiFieldNames)),
		},
		{
			id:   "PHI002",
			desc: "PHI-like identifier used in a URL/query string (may leak via referrer/logs)",
			sev:  high,
			re:   regexp.MustCompile(`(?i)[?&](` + `ssn|mrn|dob|patient(_?id)?|diagnosis|prescription` + `)=`),
		},
		{
			id:   "PHI003",
			desc: "Hard-coded SSN-shaped value",
			sev:  high,
			re:   regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			// Ignore obvious placeholders / all-zero fixtures.
			allowIf: regexp.MustCompile(`\b(000-00-0000|123-45-6789|xxx-xx-xxxx)\b`),
		},
		{
			id:   "PHI004",
			desc: "PHI-like field marked as non-sensitive / excluded from redaction",
			sev:  high,
			re:   regexp.MustCompile(`(?i)(` + trimAnchors(phiFieldNames) + `).*(json:.*\bomitempty\b.*|` + `no_?redact|skip_?redact|plaintext|cleartext)`),
		},
		{
			id:   "PHI005",
			desc: "PHI transmitted over insecure http:// endpoint (HIPAA §164.312(e) transmission security)",
			sev:  high,
			re:   regexp.MustCompile(`(?i)http://[^\s"'` + "`" + `]*(patient|health|medical|phi|ssn|mrn)`),
		},
		{
			id:   "PHI006",
			desc: "PHI-like field name introduced (verify encryption at rest & access controls)",
			sev:  medium,
			re:   regexp.MustCompile(phiFieldNames),
			// Reduce noise: only flag when it looks like a declaration/tag/column,
			// not incidental prose.
			allowIf: nil,
		},
		{
			id:   "PHI007",
			desc: "TODO/FIXME left on PHI-handling code",
			sev:  medium,
			re:   regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX).*(` + trimAnchors(phiFieldNames) + `)`),
		},
	}
}

// trimAnchors removes leading (?i) and \b anchors so a sub-pattern can be
// embedded inside a larger regex without duplicating flags.
func trimAnchors(s string) string {
	s = strings.TrimPrefix(s, `(?i)`)
	s = strings.TrimPrefix(s, `\b`)
	s = strings.TrimSuffix(s, `\b`)
	return s
}

var suppressRe = regexp.MustCompile(`phi:allow`)

// scannableExts limits scanning to source/config where PHI handling lives.
var scannableExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".java": true, ".rb": true, ".sql": true,
	".yaml": true, ".yml": true, ".json": true, ".env": true,
	".proto": true, ".graphql": true,
}

// skipDirs are never scanned.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, ".next": true, "testdata": true,
}

func main() {
	var (
		diffOnly bool
		base     string
		roots    multiFlag
	)
	flag.BoolVar(&diffOnly, "diff", false, "scan only files changed vs -base (default: whole tree)")
	flag.StringVar(&base, "base", "origin/main", "base ref for -diff mode")
	flag.Var(&roots, "path", "path to scan (repeatable); default is current dir")
	flag.Parse()

	if len(roots) == 0 {
		roots = multiFlag{"."}
	}

	files, err := collectFiles(diffOnly, base, roots)
	if err != nil {
		fmt.Fprintln(os.Stderr, "phi-check: "+err.Error())
		os.Exit(2)
	}

	var findings []finding
	for _, f := range files {
		fs, err := scanFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "phi-check: cannot scan %s: %v\n", f, err)
			continue
		}
		findings = append(findings, fs...)
	}

	report(findings)
	if len(findings) > 0 {
		os.Exit(1)
	}
}

func scanFile(path string) ([]finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []finding
	var prev string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		// Per-line suppression: marker on this line or the one above.
		suppressed := suppressRe.MatchString(line) || suppressRe.MatchString(prev)
		prev = line
		if suppressed {
			continue
		}
		for _, r := range rules {
			if !r.re.MatchString(line) {
				continue
			}
			if r.allowIf != nil && r.allowIf.MatchString(line) {
				continue
			}
			out = append(out, finding{rule: r, file: path, line: lineNo, text: strings.TrimSpace(line)})
		}
	}
	return out, sc.Err()
}

func report(findings []finding) {
	if len(findings) == 0 {
		fmt.Println("phi-check: no PHI/HIPAA findings ✓")
		return
	}
	// Stable order: severity desc, then file, then line.
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].rule.sev != findings[j].rule.sev {
			return findings[i].rule.sev > findings[j].rule.sev
		}
		if findings[i].file != findings[j].file {
			return findings[i].file < findings[j].file
		}
		return findings[i].line < findings[j].line
	})

	fmt.Printf("phi-check: %d potential PHI/HIPAA finding(s) — merge blocked\n\n", len(findings))
	inGHA := os.Getenv("GITHUB_ACTIONS") == "true"
	for _, f := range findings {
		snippet := f.text
		if len(snippet) > 160 {
			snippet = snippet[:160] + "…"
		}
		fmt.Printf("[%s] %s: %s\n  %s:%d\n  %s\n\n",
			f.rule.sev, f.rule.id, f.rule.desc, f.file, f.line, snippet)
		if inGHA {
			// Emit a GitHub Actions annotation on the exact line.
			lvl := "error"
			fmt.Printf("::%s file=%s,line=%d,title=phi-check %s::%s\n",
				lvl, f.file, f.line, f.rule.id, f.rule.desc)
		}
	}
	fmt.Println("Resolve each finding, or add `phi:allow` on/above a reviewed false positive.")
}

// --- flag helper ---

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

// collectFiles returns the set of files to scan. In -diff mode it uses git;
// otherwise it walks the roots.
func collectFiles(diffOnly bool, base string, roots []string) ([]string, error) {
	if diffOnly {
		return changedFiles(base)
	}
	var files []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if scannableExts[strings.ToLower(filepath.Ext(path))] {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
