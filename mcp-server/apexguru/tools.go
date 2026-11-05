package main

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Input bounds keep a single tool call from pinning the server.
const (
	maxScanFiles      = 100
	maxScanTotalBytes = 2 << 20 // 2 MiB of Apex per call
	maxQueryBytes     = 20000
	defaultTopCards   = 10
)

const simulatedBanner = "> **Simulated ApexGuru (CodeRabbit POV).** Org volumes and runtime telemetry are seeded demo data, not a live Salesforce org."

// scanFile is one Apex source file submitted for scanning.
type scanFile struct {
	Path    string `json:"path" jsonschema:"repo-relative path of the Apex file, e.g. salesforce/force-app/main/default/classes/Foo.cls"`
	Content string `json:"content" jsonschema:"full source text of the .cls or .trigger file"`
}

// scanArgs is the argument set for apexguru_scan.
type scanArgs struct {
	Files       []scanFile `json:"files" jsonschema:"Apex files (.cls/.trigger) to scan; non-Apex files are skipped"`
	Environment string     `json:"environment,omitempty" jsonschema:"org to project impact against: dev, uat (default), or prod"`
	Top         int        `json:"top,omitempty" jsonschema:"max detail cards to render, worst first (default 10); the summary table always lists every violation"`
}

// explainArgs is the argument set for explain_soql.
type explainArgs struct {
	Query       string `json:"query" jsonschema:"the SOQL statement, with or without surrounding [ ]"`
	Environment string `json:"environment,omitempty" jsonschema:"org to highlight: dev, uat (default), or prod; all three are always shown"`
	Context     string `json:"context,omitempty" jsonschema:"execution context: sync (default), trigger, or batch"`
}

// volumesArgs is the argument set for get_object_volumes.
type volumesArgs struct {
	Environment string `json:"environment,omitempty" jsonschema:"dev, uat, or prod; omit for all environments"`
	Object      string `json:"object,omitempty" jsonschema:"optional sObject API name, e.g. User or Signup__c"`
}

// hotspotArgs is the argument set for get_runtime_hotspots.
type hotspotArgs struct {
	ClassName   string `json:"class_name" jsonschema:"Apex class name without extension, e.g. ACME_LeadTriggerHandler"`
	Environment string `json:"environment,omitempty" jsonschema:"uat (default) or prod"`
}

// scan runs static detection over the files and enriches each violation with
// projected impact in env.
func (s *seedData) scan(args scanArgs) (string, error) {
	env, err := validEnv(args.Environment)
	if err != nil {
		return "", err
	}
	if len(args.Files) == 0 {
		return "", fmt.Errorf("files is required: pass at least one {path, content} Apex file")
	}
	if len(args.Files) > maxScanFiles {
		return "", fmt.Errorf("too many files: %d (max %d per call); split the scan", len(args.Files), maxScanFiles)
	}
	total := 0
	for _, f := range args.Files {
		total += len(f.Content)
	}
	if total > maxScanTotalBytes {
		return "", fmt.Errorf("files total %d bytes (max %d per call); split the scan", total, maxScanTotalBytes)
	}
	top := args.Top
	if top <= 0 {
		top = defaultTopCards
	}

	var vs []Violation
	scanned, skipped := 0, []string{}
	for _, f := range args.Files {
		ext := strings.ToLower(path.Ext(f.Path))
		if ext != ".cls" && ext != ".trigger" {
			skipped = append(skipped, f.Path)
			continue
		}
		scanned++
		af := parseApex(f.Path, f.Content)
		for _, v := range af.detect() {
			vs = append(vs, s.enrich(v, env))
		}
		vs = append(vs, s.volumeViolations(af, env)...)
	}
	sort.SliceStable(vs, func(i, j int) bool { return vs[i].Severity < vs[j].Severity })
	return s.renderScan(vs, env, scanned, skipped, top), nil
}

// volumeViolations flags queries that have a WHERE clause (so the static
// rules pass them) but still fail at env's volumes. It needs org data, so it
// never fires for dev or for isolated test classes.
func (s *seedData) volumeViolations(f *apexFile, env string) []Violation {
	if env == "dev" || f.isTest {
		return nil
	}
	ctx := "sync"
	base := strings.TrimSuffix(path.Base(f.path), path.Ext(f.path))
	if strings.EqualFold(path.Ext(f.path), ".trigger") || strings.HasSuffix(strings.ToLower(base), "triggerhandler") {
		ctx = "trigger"
	}
	var out []Violation
	for _, q := range f.soql {
		text := f.src[q.start:q.end]
		parts, err := splitSOQL(text)
		if err != nil || parts.Where == "" {
			continue // unbounded queries are covered by SOQL_NO_WHERE_OR_LIMIT
		}
		p, err := s.explainSOQL(text, env, ctx)
		if err != nil || !p.Known || p.Pass() {
			continue
		}
		devRows, _ := s.rows("dev", p.Object)
		v := Violation{
			Rule: ruleNonSelective, Severity: ruleNonSelective.Severity, File: f.path, Line: f.lineOf(q.start),
			Method: f.methodAt(q.start), Code: f.lineText(q.start), Query: text, Object: p.Object,
			Detail: fmt.Sprintf("Planner: %s on %s (%s context). %s", p.Plan, p.Object, ctx, strings.Join(p.Warnings, " ")),
			Insight: fmt.Sprintf("%s has %s rows in dev (passes) and %s rows in %s. %s",
				p.Object, commas(devRows), commas(p.ObjectRows), env, strings.Join(p.Failures, " ")),
		}
		if hs := s.Hotspots[base][env]; len(hs) > 0 {
			for _, h := range hs {
				if h.Note != "" && strings.EqualFold(h.Method, v.Method) {
					v.Insight += " Runtime telemetry: " + h.Note
				}
			}
		}
		out = append(out, v)
	}
	return out
}

// enrich adds projected production impact for env and escalates severity
// where the seeded volumes show the code would fail.
func (s *seedData) enrich(v Violation, env string) Violation {
	if env == "dev" || v.IsTest {
		return v
	}
	class := strings.TrimSuffix(path.Base(v.File), path.Ext(v.File))
	switch v.Rule.Code {
	case "SOQL_IN_LOOP", "SOQL_IN_LOOP_1HOP":
		v.Severity = SevCritical
		v.Insight = fmt.Sprintf("A %d-record trigger or API batch in %s issues %d queries against a synchronous limit of %d, so the transaction fails with System.LimitException: Too many SOQL queries: %d.",
			triggerBatchSize, env, triggerBatchSize, syncSOQLQueryLimit, syncSOQLQueryLimit+1)
		if v.Query != "" {
			if p, err := s.explainSOQL(v.Query, env, "sync"); err == nil && p.Known {
				if p.Selective {
					v.Insight += fmt.Sprintf(" Each query is selective (Index on %s, %s rows), so the failure is query count, not volume.", p.Object, commas(p.ObjectRows))
				} else {
					v.Insight += fmt.Sprintf(" Each query is also a full scan of %s %s rows.", commas(p.ObjectRows), p.Object)
				}
			}
		}
	case "DML_IN_LOOP", "DML_IN_LOOP_1HOP":
		v.Insight = fmt.Sprintf("A %d-record batch in %s issues %d DML statements against a limit of %d, and every statement re-fires Lead triggers and flows.",
			triggerBatchSize, env, triggerBatchSize, syncDMLStatementLimit)
	case "SOQL_NO_WHERE_OR_LIMIT":
		if p, err := s.explainSOQL(v.Query, env, "sync"); err == nil && p.Known {
			devRows, _ := s.rows("dev", p.Object)
			v.Insight = fmt.Sprintf("%s has %s rows in dev and %s rows in %s.", p.Object, commas(devRows), commas(p.ObjectRows), env)
			if !p.Pass() {
				v.Severity = SevCritical
				v.Insight += " " + strings.Join(p.Failures, " ") + " The query passes in dev and fails after promotion."
			}
			if note := s.Notes[p.Object]; note != "" {
				v.Insight += " " + note
			}
		}
	case "GGD":
		v.Insight = fmt.Sprintf("The %s org carries managed packages and many more custom objects than dev, so each call costs proportionally more CPU and heap.", env)
	}
	if hs := s.Hotspots[class][env]; len(hs) > 0 && strings.HasPrefix(v.Rule.Code, "SOQL") {
		for _, h := range hs {
			if h.Note != "" && (v.Method == "" || strings.EqualFold(h.Method, v.Method)) {
				v.Insight += " Runtime telemetry: " + h.Note
			}
		}
	}
	return v
}

// renderScan prints the --present style report: legend, worst-first issue
// cards (capped at top), and a summary table listing every violation.
func (s *seedData) renderScan(vs []Violation, env string, scanned int, skipped []string, top int) string {
	var b strings.Builder
	mode := fmt.Sprintf("Production insights (simulated %s)", env)
	if env == "dev" {
		mode = "Static only"
	}
	fmt.Fprintf(&b, "## ApexGuru Scan Complete: %s\n\n%s\n\n", mode, simulatedBanner)
	files := map[string]bool{}
	for _, v := range vs {
		files[v.File] = true
	}
	fmt.Fprintf(&b, "**Found %d performance violations** across %d of %d scanned Apex files.\n\n", len(vs), len(files), scanned)
	if len(skipped) > 0 {
		fmt.Fprintf(&b, "Skipped non-Apex files: %s\n\n", strings.Join(skipped, ", "))
	}
	if len(vs) == 0 {
		fmt.Fprintf(&b, "No performance antipatterns found. Projected against %s volumes.\n", env)
		return b.String()
	}
	b.WriteString("**Severity legend:** Critical (1) fails a governor limit at target volumes · High (2) likely limit failure under load · Moderate (3) scales poorly with org size · Low (4) waste\n\n")

	n := len(vs)
	if n > top {
		fmt.Fprintf(&b, "### Top %d of %d issues (worst first)\n\n", top, n)
		n = top
	}
	for i, v := range vs[:n] {
		fmt.Fprintf(&b, "### Issue %d: %s · %s\n\n", i+1, v.Rule.Key, v.Severity)
		loc := fmt.Sprintf("`%s:%d`", v.File, v.Line)
		if v.Method != "" {
			loc += fmt.Sprintf(" in `%s()`", v.Method)
		}
		fmt.Fprintf(&b, "- **Location:** %s\n- **Why:** %s\n", loc, v.Rule.Why)
		if v.Detail != "" {
			fmt.Fprintf(&b, "- **Detail:** %s\n", v.Detail)
		}
		if v.Insight != "" {
			fmt.Fprintf(&b, "- **Production insight (%s):** %s\n", env, v.Insight)
		}
		fmt.Fprintf(&b, "- **Suggested fix:** %s\n- **Resource:** %s\n\n```apex\n%s\n```\n\n", v.Rule.Fix, v.Rule.Resource, v.Code)
	}

	b.WriteString("## Summary\n\n| # | Rule | Severity | File:Line | Method |\n|---|------|----------|-----------|--------|\n")
	for i, v := range vs {
		fmt.Fprintf(&b, "| %d | %s | %s | %s:%d | %s |\n", i+1, v.Rule.Key, v.Severity, v.File, v.Line, v.Method)
	}
	counts := map[Severity]int{}
	for _, v := range vs {
		counts[v.Severity]++
	}
	fmt.Fprintf(&b, "\nCritical: %d · High: %d · Moderate: %d · Low: %d\n", counts[SevCritical], counts[SevHigh], counts[SevModerate], counts[SevLow])
	return b.String()
}

// explain renders the explain_soql report: one row per environment so the
// "works in dev, fails in UAT" gap is visible at a glance.
func (s *seedData) explain(args explainArgs) (string, error) {
	env, err := validEnv(args.Environment)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(args.Query) == "" {
		return "", fmt.Errorf("query is required")
	}
	if len(args.Query) > maxQueryBytes {
		return "", fmt.Errorf("query is %d bytes (max %d)", len(args.Query), maxQueryBytes)
	}
	ctx := strings.ToLower(strings.TrimSpace(args.Context))
	if ctx == "" {
		ctx = "sync"
	}
	if ctx != "sync" && ctx != "trigger" && ctx != "batch" {
		return "", fmt.Errorf("unknown context %q; expected sync, trigger, or batch", args.Context)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## SOQL plan: projected across orgs (%s context)\n\n%s\n\n```sql\n%s\n```\n\n", ctx, simulatedBanner, strings.TrimSpace(args.Query))
	b.WriteString("| Org | Object rows | Plan | Est. rows returned | Verdict |\n|-----|-------------|------|--------------------|---------|\n")
	var focus soqlPlan
	for _, e := range environments {
		p, err := s.explainSOQL(args.Query, e, ctx)
		if err != nil {
			return "", err
		}
		if e == env {
			focus = p
		}
		verdict := "PASS"
		if !p.Known {
			verdict = "UNKNOWN"
		} else if !p.Pass() {
			verdict = "**FAIL**"
		}
		name := e
		if e == env {
			name = "**" + e + "**"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", name, commas(p.ObjectRows), p.Plan, commas(p.EstimatedRows), verdict)
	}
	if len(focus.Failures) > 0 {
		fmt.Fprintf(&b, "\n**Failures in %s:**\n", env)
		for _, f := range focus.Failures {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	if len(focus.Warnings) > 0 {
		b.WriteString("\n**Planner notes:**\n")
		for _, w := range focus.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}
	if note := s.Notes[focus.Object]; note != "" {
		fmt.Fprintf(&b, "\n_%s_\n", note)
	}
	if !focus.Selective && focus.Known {
		fmt.Fprintf(&b, "\n**Fix:** filter on an indexed field of %s (%s) and/or add a LIMIT.\n", focus.Object, strings.Join(s.indexedFields(focus.Object), ", "))
	}
	return b.String(), nil
}

// indexedFields lists an object's indexed fields in stable order.
func (s *seedData) indexedFields(object string) []string {
	var out []string
	for f := range s.Indexed[object] {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// volumes renders seeded row counts.
func (s *seedData) volumes(args volumesArgs) (string, error) {
	envs := environments
	if strings.TrimSpace(args.Environment) != "" {
		e, err := validEnv(args.Environment)
		if err != nil {
			return "", err
		}
		envs = []string{e}
	}
	objects := s.sortedObjects()
	if strings.TrimSpace(args.Object) != "" {
		c, ok := s.canonicalObject(args.Object)
		if !ok {
			return "", fmt.Errorf("unknown object %q; known objects: %s", args.Object, strings.Join(objects, ", "))
		}
		objects = []string{c}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Object volumes\n\n%s\n\n| Object |", simulatedBanner)
	for _, e := range envs {
		fmt.Fprintf(&b, " %s |", e)
	}
	b.WriteString(" Indexed fields |\n|---|")
	for range envs {
		b.WriteString("---|")
	}
	b.WriteString("---|\n")
	for _, o := range objects {
		fmt.Fprintf(&b, "| %s |", o)
		for _, e := range envs {
			r, _ := s.rows(e, o)
			fmt.Fprintf(&b, " %s |", commas(r))
		}
		fmt.Fprintf(&b, " %s |\n", strings.Join(s.indexedFields(o), ", "))
	}
	return b.String(), nil
}

// governorLimits renders the Apex per-transaction limits the reviewer should
// check against.
func governorLimits() string {
	return `## Apex governor limits (per transaction)

| Limit | Synchronous | Asynchronous |
|-------|-------------|--------------|
| SOQL queries issued | 100 | 200 |
| Rows retrieved by SOQL | 50,000 | 50,000 |
| Rows from Database.getQueryLocator (Batch start) | 10,000 | 50,000,000 |
| SOSL queries | 20 | 20 |
| DML statements | 150 | 150 |
| Rows processed by DML | 10,000 | 10,000 |
| CPU time | 10,000 ms | 60,000 ms |
| Heap size | 6 MB | 12 MB |
| Callouts (HTTP / web service) | 100 | 100 |
| Cumulative callout timeout | 120 s | 120 s |
| @future calls | 50 | 0 in batch/future |
| System.enqueueJob (Queueable) | 50 | 1 |
| Trigger batch size (records per chunk) | 200 | 200 |
| Non-selective query threshold in triggers | 200,000 rows | 200,000 rows |

Source: https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_gov_limits.htm
`
}

// hotspots renders seeded runtime telemetry for one class.
func (s *seedData) hotspots(args hotspotArgs) (string, error) {
	env, err := validEnv(args.Environment)
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(args.ClassName)
	if name == "" {
		return "", fmt.Errorf("class_name is required")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Runtime hotspots: %s (%s)\n\n%s\n\n", name, env, simulatedBanner)
	canon, ok := s.hotspotNames[strings.ToLower(name)]
	rows := s.Hotspots[canon][env]
	if !ok || len(rows) == 0 {
		fmt.Fprintf(&b, "No runtime telemetry for `%s` in %s. The class has not executed there (new or unpromoted code). Use `apexguru_scan` and `explain_soql` for projected impact.\n", name, env)
		return b.String(), nil
	}
	b.WriteString("| Method | Avg CPU ms | p95 CPU ms | Avg heap KB | p95 SOQL | p95 DML | Calls/day | Exceptions (7d) |\n|---|---|---|---|---|---|---|---|\n")
	for _, h := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %d/100 | %d/150 | %s | %d |\n",
			h.Method, commas(int64(h.AvgCPUMs)), commas(int64(h.P95CPUMs)), commas(int64(h.AvgHeapKB)),
			h.P95SOQLQueries, h.P95DMLStatements, commas(int64(h.InvocationsPerDay)), h.Exceptions7d)
	}
	for _, h := range rows {
		if h.Note != "" {
			fmt.Fprintf(&b, "\n- **%s:** %s\n", h.Method, h.Note)
		}
	}
	return b.String(), nil
}
