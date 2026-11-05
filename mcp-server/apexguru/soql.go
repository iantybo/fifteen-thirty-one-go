package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Governor limits that the simulated query planner checks against.
const (
	queryRowLimit          = 50000  // total rows retrieved by SOQL per transaction
	nonSelectiveThreshold  = 200000 // objects above this throw on non-selective trigger queries
	triggerBatchSize       = 200    // records per trigger chunk
	syncSOQLQueryLimit     = 100    // SOQL queries per synchronous transaction
	syncDMLStatementLimit  = 150    // DML statements per transaction
	defaultBindListEntries = triggerBatchSize
)

// soqlPlan is the simulated planner's verdict for one query in one org.
type soqlPlan struct {
	Env           string
	Object        string
	ObjectRows    int64
	Known         bool // object present in the seed
	Plan          string
	Selective     bool
	EstimatedRows int64
	Limit         int64 // 0 = no LIMIT clause
	Failures      []string
	Warnings      []string
}

// Pass reports whether the query is expected to run without a limit exception.
func (p soqlPlan) Pass() bool { return len(p.Failures) == 0 }

var (
	reLimit      = regexp.MustCompile(`(?i)^\s*(\d+|:\s*\w+)`)
	reComparison = regexp.MustCompile(`(?i)([A-Za-z_][\w.]*)\s*(!=|<>|<=|>=|=|<|>|\bNOT\s+IN\b|\bIN\b|\bNOT\s+LIKE\b|\bLIKE\b|\bINCLUDES\b|\bEXCLUDES\b)\s*('(?:[^'\\]|\\.)*'|\([^)]*\)|:\s*[\w.()]+|[\w.:-]+)`)
	reWord       = regexp.MustCompile(`\S+`)
	reWS         = regexp.MustCompile(`\s+`)
)

// topLevelKeyword returns the index of the first occurrence of kw (a phrase,
// matched case-insensitively on word boundaries) at parenthesis depth zero and
// outside string literals, searching from start. It returns -1 if absent.
func topLevelKeyword(q string, kw string, start int) int {
	lower := strings.ToLower(q)
	kw = strings.ToLower(kw)
	depth := 0
	inStr := false
	for i := start; i < len(q); i++ {
		c := q[i]
		if inStr {
			if c == '\\' {
				i++
			} else if c == '\'' {
				inStr = false
			}
			continue
		}
		switch c {
		case '\'':
			inStr = true
			continue
		case '(':
			depth++
			continue
		case ')':
			depth--
			continue
		}
		if depth != 0 || !strings.HasPrefix(lower[i:], kw) {
			continue
		}
		before := i == 0 || !isIdent(q[i-1])
		after := i+len(kw) >= len(q) || !isIdent(q[i+len(kw)])
		if before && after {
			return i
		}
	}
	return -1
}

func isIdent(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// soqlParts is the outer query split into its clauses.
type soqlParts struct {
	SelectList string
	Object     string
	Where      string
	LimitRaw   string // literal digits, a bind, or ""
}

// splitSOQL extracts the outer FROM object, WHERE clause and LIMIT of a SOQL
// statement. Subqueries in the select list are skipped because only depth-zero
// keywords are considered.
func splitSOQL(q string) (soqlParts, error) {
	q = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(q), "["), "]"))
	sel := topLevelKeyword(q, "select", 0)
	from := topLevelKeyword(q, "from", 0)
	if sel < 0 || from < 0 || from < sel {
		return soqlParts{}, fmt.Errorf("not a SOQL SELECT ... FROM statement")
	}
	var p soqlParts
	p.SelectList = strings.TrimSpace(q[sel+len("select") : from])
	rest := strings.TrimSpace(q[from+len("from"):])
	if f := reWord.FindString(rest); f != "" {
		p.Object = strings.TrimRight(f, ",")
	}
	if p.Object == "" {
		return soqlParts{}, fmt.Errorf("SOQL statement has no object after FROM")
	}

	clauseEnd := func(from int) int {
		end := len(q)
		for _, kw := range []string{"with", "group by", "order by", "limit", "offset", "for update", "for view", "for reference", "all rows"} {
			if i := topLevelKeyword(q, kw, from); i >= 0 && i < end {
				end = i
			}
		}
		return end
	}
	if w := topLevelKeyword(q, "where", from); w >= 0 {
		p.Where = strings.TrimSpace(q[w+len("where") : clauseEnd(w+len("where"))])
	}
	if l := topLevelKeyword(q, "limit", from); l >= 0 {
		if m := reLimit.FindStringSubmatch(q[l+len("limit"):]); m != nil {
			p.LimitRaw = reWS.ReplaceAllString(m[1], "")
		}
	}
	return p, nil
}

// explainSOQL simulates the Salesforce query optimizer for query in env using
// the seeded object volumes and indexes. context is "sync", "trigger" or
// "batch" and changes which limits apply.
func (s *seedData) explainSOQL(query, env, context string) (soqlPlan, error) {
	parts, err := splitSOQL(query)
	if err != nil {
		return soqlPlan{}, err
	}
	plan := soqlPlan{Env: env, Object: parts.Object}
	rows, known := s.rows(env, parts.Object)
	plan.Known = known
	if !known {
		plan.Plan = "Unknown"
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s has no seeded volume data in %s; cannot estimate rows.", parts.Object, env))
		return plan, nil
	}
	if c, ok := s.canonicalObject(parts.Object); ok {
		plan.Object = c
	}
	plan.ObjectRows = rows

	est := float64(rows)
	plan.Plan = "TableScan"
	if parts.Where != "" {
		sel, frac, reasons := s.whereSelectivity(plan.Object, parts.Where, rows)
		plan.Warnings = append(plan.Warnings, reasons...)
		if sel {
			plan.Selective = true
			plan.Plan = "Index"
			est = math.Max(1, math.Round(float64(rows)*frac))
		}
	}

	switch {
	case parts.LimitRaw == "":
	case strings.HasPrefix(parts.LimitRaw, ":"):
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("LIMIT is a bind variable (%s); the cap depends on runtime input.", parts.LimitRaw))
	default:
		if n, err := strconv.ParseInt(parts.LimitRaw, 10, 64); err == nil {
			plan.Limit = n
			est = math.Min(est, float64(n))
		}
	}
	plan.EstimatedRows = int64(est)

	if context != "batch" && plan.EstimatedRows > queryRowLimit {
		plan.Failures = append(plan.Failures, fmt.Sprintf(
			"System.LimitException: Too many query rows: %d. Query returns ~%s rows; the transaction limit is %s.",
			queryRowLimit+1, commas(plan.EstimatedRows), commas(queryRowLimit)))
	}
	if context == "trigger" && !plan.Selective && rows > nonSelectiveThreshold {
		plan.Failures = append(plan.Failures, fmt.Sprintf(
			"System.QueryException: Non-selective query against large object type (more than %s rows). %s has %s rows in %s.",
			commas(nonSelectiveThreshold), plan.Object, commas(rows), env))
	}
	if parts.Where == "" && parts.LimitRaw == "" {
		plan.Warnings = append(plan.Warnings, "No WHERE and no LIMIT: the query reads the entire object, so it grows with org data volume.")
	}
	return plan, nil
}

// whereSelectivity decides whether a WHERE clause can use an index. A clause
// is selective when at least one indexed field is filtered with a positive
// operator, and (when OR is present) every filtered field is indexed. frac is
// the estimated fraction of the object's rows returned.
func (s *seedData) whereSelectivity(object, where string, rows int64) (selective bool, frac float64, reasons []string) {
	matches := reComparison.FindAllStringSubmatch(where, -1)
	if len(matches) == 0 {
		return false, 1, []string{"WHERE clause has no recognizable field comparisons; assuming a full scan."}
	}
	hasOR := topLevelKeyword(where, "or", 0) >= 0
	best := 1.0
	anyIndexed, allIndexed := false, true
	for _, m := range matches {
		field := m[1]
		if i := strings.LastIndex(field, "."); i >= 0 {
			field = field[i+1:]
		}
		op := strings.ToUpper(reWS.ReplaceAllString(m[2], " "))
		val := strings.TrimSpace(m[3])
		fieldFrac, indexed := s.indexedFraction(object, field)
		if !indexed {
			allIndexed = false
			reasons = append(reasons, fmt.Sprintf("%s.%s is not indexed; filtering on it cannot use an index.", object, field))
			continue
		}
		switch {
		case op == "!=" || op == "<>" || op == "NOT IN" || op == "NOT LIKE" || op == "EXCLUDES":
			allIndexed = false
			reasons = append(reasons, fmt.Sprintf("Negative operator %s on %s is never selective.", op, field))
			continue
		case strings.EqualFold(val, "null"):
			allIndexed = false
			reasons = append(reasons, fmt.Sprintf("Comparing %s to null cannot use the index.", field))
			continue
		case op == "LIKE" && strings.HasPrefix(strings.Trim(val, "'"), "%"):
			allIndexed = false
			reasons = append(reasons, fmt.Sprintf("Leading wildcard in LIKE on %s defeats the index.", field))
			continue
		}
		anyIndexed = true
		f := fieldFrac
		switch op {
		case "IN":
			n := defaultBindListEntries
			if strings.HasPrefix(val, "(") {
				n = strings.Count(val, ",") + 1
			}
			if f == 0 {
				f = math.Min(1, float64(n)/math.Max(1, float64(rows))) // unique field: one row per entry
			} else {
				f = math.Min(1, f*float64(n))
			}
		case "<", ">", "<=", ">=":
			f = math.Max(f, 0.05) // ranges select a slice of the table
		}
		if f < best {
			best = f
		}
	}
	if hasOR && !allIndexed {
		reasons = append(reasons, "OR with a non-indexed branch forces a full scan.")
		return false, 1, reasons
	}
	if !anyIndexed {
		return false, 1, reasons
	}
	return true, best, reasons
}

// commas formats n with thousands separators.
func commas(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
