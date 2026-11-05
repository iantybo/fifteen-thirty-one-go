package main

import (
	"regexp"
	"sort"
	"strings"
)

// Severity follows ApexGuru's 1-5 scale: 1 = Critical ... 5 = Info.
type Severity int

const (
	SevCritical Severity = 1
	SevHigh     Severity = 2
	SevModerate Severity = 3
	SevLow      Severity = 4
)

func (s Severity) String() string {
	switch s {
	case SevCritical:
		return "Critical (1)"
	case SevHigh:
		return "High (2)"
	case SevModerate:
		return "Moderate (3)"
	case SevLow:
		return "Low (4)"
	default:
		return "Info (5)"
	}
}

// Rule identifiers mirror the ApexGuru violation catalog. Key is the
// human-readable name presented to reviewers; Code is the API rule code.
type Rule struct {
	Key      string
	Code     string
	Why      string
	Fix      string
	Resource string
	Severity Severity
}

var (
	ruleSOQLInLoop = Rule{
		Key: "SoqlInALoop", Code: "SOQL_IN_LOOP", Severity: SevHigh,
		Why:      "A SOQL query runs on every loop iteration. Synchronous transactions allow 100 queries, so a 200-record trigger batch fails with System.LimitException.",
		Fix:      "Query once before the loop (bind a Set<Id>), build a Map<Id, SObject>, and read from the map inside the loop.",
		Resource: "https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_gov_limits.htm",
	}
	ruleSOQLInLoopOneHop = Rule{
		Key: "SoqlInALoopOneHop", Code: "SOQL_IN_LOOP_1HOP", Severity: SevHigh,
		Why:      "A method called inside this loop issues SOQL, so the query still runs once per iteration.",
		Fix:      "Bulkify the called method to accept a collection, call it once outside the loop, and look results up from a Map.",
		Resource: "https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_gov_limits.htm",
	}
	ruleDMLInLoop = Rule{
		Key: "DmlInALoop", Code: "DML_IN_LOOP", Severity: SevCritical,
		Why:      "A DML statement runs on every loop iteration. Transactions allow 150 DML statements, and each one also re-fires triggers, flows, and validation.",
		Fix:      "Collect records in a List inside the loop and perform one DML statement after the loop.",
		Resource: "https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_triggers_bulk_idioms.htm",
	}
	ruleDMLInLoopOneHop = Rule{
		Key: "DmlInALoopOneHop", Code: "DML_IN_LOOP_1HOP", Severity: SevCritical,
		Why:      "A method called inside this loop performs DML, so the DML statement still runs once per iteration.",
		Fix:      "Have the called method return records instead of saving them, then perform one DML after the loop.",
		Resource: "https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_triggers_bulk_idioms.htm",
	}
	ruleGGD = Rule{
		Key: "SchemaGetGlobalDescribeNotEfficient", Code: "GGD", Severity: SevModerate,
		Why:      "Schema.getGlobalDescribe() describes every object in the org. Its cost grows with org size, and UAT and prod have far more metadata than a dev sandbox.",
		Fix:      "Use a targeted describe such as Lead.SObjectType.getDescribe() or Schema.describeSObjects(new List<String>{'Lead'}).",
		Resource: "https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_dynamic_describe_objects_understanding.htm",
	}
	ruleNoWhere = Rule{
		Key: "SoqlWithoutWhereOrLimit", Code: "SOQL_NO_WHERE_OR_LIMIT", Severity: SevModerate,
		Why:      "The query has neither WHERE nor LIMIT, so it reads the whole object. It works on small sandboxes and fails once volumes grow.",
		Fix:      "Add a selective WHERE clause on an indexed field and/or a LIMIT.",
		Resource: "https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/langCon_apex_SOQL_VLSQ.htm",
	}
	ruleNonSelective = Rule{
		Key: "SoqlNonSelectiveAtVolume", Code: "SOQL_NON_SELECTIVE", Severity: SevCritical,
		Why:      "The query has a WHERE clause, but no filter can use an index. It passes on small sandboxes and fails at UAT/prod volumes. This is detectable only with org data volumes, not from source alone.",
		Fix:      "Filter on an indexed field (Id, Name, OwnerId, CreatedDate, lookups, or external IDs), add a LIMIT, or move high-volume work to Batch Apex.",
		Resource: "https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/langCon_apex_SOQL_VLSQ.htm",
	}
	ruleUnusedFields = Rule{
		Key: "SoqlUnusedFields", Code: "SOQL_UNUSED_FIELDS", Severity: SevLow,
		Why:      "Selected fields are never read, which wastes query CPU and heap. Unused PII fields also widen data exposure.",
		Fix:      "Select only the fields the code reads.",
		Resource: "https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/langCon_apex_SOQL_VLSQ.htm",
	}
)

// Violation is one detected antipattern instance.
type Violation struct {
	Rule     Rule
	Severity Severity
	File     string
	Line     int
	Method   string
	Code     string // the offending source line, trimmed
	Detail   string // rule-specific detail (fields, object)
	Insight  string // production insight added by enrichment
	Query    string // SOQL text when the rule concerns a query
	Object   string // FROM object when known
	IsTest   bool   // found in an isolated-data test class; skip volume projection
}

type span struct{ start, end int } // [start, end)

func (s span) contains(i int) bool { return i >= s.start && i < s.end }

// apexFile is a parsed Apex source file.
type apexFile struct {
	path    string
	isTest  bool   // @IsTest class without SeeAllData=true: runs against isolated test data
	src     string // original
	clean   string // comments and string contents blanked; same length as src
	lines   []int  // byte offset of each line start
	loops   []span
	methods []methodSpan
	soql    []span // [ ... ] static SOQL expressions
	dml     []int  // positions of DML keywords / Database.<dml>( calls
}

type methodSpan struct {
	name    string
	namePos int
	body    span
}

var (
	reLoopHeader  = regexp.MustCompile(`(?i)\b(for|while)\s*\(`)
	reDoLoop      = regexp.MustCompile(`(?i)\bdo\s*\{`)
	reSOQLStart   = regexp.MustCompile(`(?i)\[\s*select\b`)
	reDMLStmt     = regexp.MustCompile(`(?im)(?:^|[;{})])\s*(insert|update|delete|upsert|undelete|merge)\s+[\w\[(]`)
	reDMLDatabase = regexp.MustCompile(`(?i)\bDatabase\s*\.\s*(insert|update|delete|upsert|undelete|merge)\s*\(`)
	reDynQuery    = regexp.MustCompile(`(?i)\bDatabase\s*\.\s*(query|queryWithBinds)\s*\(`)
	reGGD         = regexp.MustCompile(`(?i)\bSchema\s*\.\s*getGlobalDescribe\s*\(`)
	reMethodDecl  = regexp.MustCompile(`(?i)\b([A-Za-z_]\w*)\s*\((?:[^()]|\([^()]*\))*\)\s*(?:throws\s+[\w.,\s]+)?\{`)
	reReturnTail  = regexp.MustCompile(`(?i)\breturn\s*$`)
	reTestClass   = regexp.MustCompile(`(?i)@isTest(?:\s*\([^)]*\))?\s+(?:\w+\s+)*class\b`)
	reSeeAllData  = regexp.MustCompile(`(?i)SeeAllData\s*=\s*true`)
)

var nonMethodWords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true, "when": true,
	"do": true, "else": true, "try": true, "finally": true, "return": true, "new": true,
	"super": true, "this": true, "on": true, "trigger": true,
}

// sanitize blanks comments and the contents of string literals while
// preserving length and newlines, so offsets map back to the original source.
func sanitize(src string) string {
	b := []byte(src)
	for i := 0; i < len(b); i++ {
		switch {
		case b[i] == '/' && i+1 < len(b) && b[i+1] == '/':
			for ; i < len(b) && b[i] != '\n'; i++ {
				b[i] = ' '
			}
		case b[i] == '/' && i+1 < len(b) && b[i+1] == '*':
			b[i], b[i+1] = ' ', ' '
			for i += 2; i < len(b) && !(b[i] == '*' && i+1 < len(b) && b[i+1] == '/'); i++ {
				if b[i] != '\n' {
					b[i] = ' '
				}
			}
			if i+1 < len(b) {
				b[i], b[i+1] = ' ', ' '
				i++
			}
		case b[i] == '\'':
			for i++; i < len(b) && b[i] != '\'' && b[i] != '\n'; i++ {
				if b[i] == '\\' && i+1 < len(b) {
					b[i] = ' '
					i++
				}
				b[i] = ' '
			}
		}
	}
	return string(b)
}

// match returns the index of the bracket closing the one at open, or -1.
func match(s string, open int) int {
	o := s[open]
	var c byte
	switch o {
	case '(':
		c = ')'
	case '{':
		c = '}'
	case '[':
		c = ']'
	default:
		return -1
	}
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case o:
			depth++
		case c:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseApex builds loop, method, SOQL, and DML indexes for one file.
func parseApex(path, src string) *apexFile {
	f := &apexFile{path: path, src: src, clean: sanitize(src), lines: []int{0}}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			f.lines = append(f.lines, i+1)
		}
	}
	c := f.clean
	f.isTest = reTestClass.MatchString(c) && !reSeeAllData.MatchString(c)

	for _, m := range reLoopHeader.FindAllStringIndex(c, -1) {
		openParen := m[1] - 1
		closeParen := match(c, openParen)
		if closeParen < 0 {
			continue
		}
		r := closeParen + 1
		for r < len(c) && (c[r] == ' ' || c[r] == '\t' || c[r] == '\n' || c[r] == '\r') {
			r++
		}
		if r >= len(c) {
			continue
		}
		if c[r] == '{' {
			if e := match(c, r); e > 0 {
				f.loops = append(f.loops, span{r, e})
			}
		} else if semi := strings.IndexByte(c[r:], ';'); semi > 0 {
			f.loops = append(f.loops, span{r, r + semi + 1}) // single-statement body
		}
	}
	for _, m := range reDoLoop.FindAllStringIndex(c, -1) {
		if e := match(c, m[1]-1); e > 0 {
			f.loops = append(f.loops, span{m[1] - 1, e})
		}
	}

	for _, m := range reMethodDecl.FindAllStringSubmatchIndex(c, -1) {
		name := c[m[2]:m[3]]
		if nonMethodWords[strings.ToLower(name)] {
			continue
		}
		if e := match(c, m[1]-1); e > 0 {
			f.methods = append(f.methods, methodSpan{name: name, namePos: m[2], body: span{m[1] - 1, e}})
		}
	}

	for _, m := range reSOQLStart.FindAllStringIndex(c, -1) {
		if e := match(c, m[0]); e > 0 {
			f.soql = append(f.soql, span{m[0], e + 1})
		}
	}
	for _, m := range reDMLStmt.FindAllStringSubmatchIndex(c, -1) {
		f.dml = append(f.dml, m[2])
	}
	for _, m := range reDMLDatabase.FindAllStringIndex(c, -1) {
		f.dml = append(f.dml, m[0])
	}
	return f
}

// lineOf converts a byte offset to a 1-based line number.
func (f *apexFile) lineOf(pos int) int {
	return sort.Search(len(f.lines), func(i int) bool { return f.lines[i] > pos })
}

// lineText returns the trimmed original source line containing pos.
func (f *apexFile) lineText(pos int) string {
	n := f.lineOf(pos)
	start := f.lines[n-1]
	end := len(f.src)
	if n < len(f.lines) {
		end = f.lines[n] - 1
	}
	return strings.TrimSpace(f.src[start:end])
}

func (f *apexFile) inLoop(pos int) bool {
	for _, l := range f.loops {
		if l.contains(pos) {
			return true
		}
	}
	return false
}

// methodAt returns the innermost method containing pos, or "".
func (f *apexFile) methodAt(pos int) string {
	best, size := "", -1
	for _, m := range f.methods {
		if m.body.contains(pos) && (size < 0 || m.body.end-m.body.start < size) {
			best, size = m.name, m.body.end-m.body.start
		}
	}
	return best
}

// hasOp reports whether any position in ops falls inside s.
func hasOp(s span, soql []span, dml []int) (hasSOQL, hasDML bool) {
	for _, q := range soql {
		if s.contains(q.start) {
			hasSOQL = true
		}
	}
	for _, d := range dml {
		if s.contains(d) {
			hasDML = true
		}
	}
	return hasSOQL, hasDML
}

// detect runs every static rule over the file.
func (f *apexFile) detect() []Violation {
	var out []Violation
	add := func(r Rule, pos int, detail, query, object string) {
		out = append(out, Violation{
			Rule: r, Severity: r.Severity, File: f.path, Line: f.lineOf(pos),
			Method: f.methodAt(pos), Code: f.lineText(pos), Detail: detail, Query: query, Object: object,
			IsTest: f.isTest,
		})
	}

	dynQueries := reDynQuery.FindAllStringIndex(f.clean, -1)
	for _, q := range f.soql {
		text := f.src[q.start:q.end]
		parts, err := splitSOQL(text)
		obj := ""
		if err == nil {
			obj = parts.Object
		}
		if f.inLoop(q.start) {
			add(ruleSOQLInLoop, q.start, "", text, obj)
		}
		// Isolated test data never reaches org volumes, so unbounded queries in tests are fine.
		if err == nil && !f.isTest && parts.Where == "" && parts.LimitRaw == "" {
			add(ruleNoWhere, q.start, "", text, obj)
		}
		if err == nil && !reReturnTail.MatchString(f.clean[:q.start]) {
			if unused := f.unusedFields(parts.SelectList, q); len(unused) > 0 {
				add(ruleUnusedFields, q.start, "Unused fields: "+strings.Join(unused, ", "), text, obj)
			}
		}
	}
	for _, d := range dynQueries {
		if f.inLoop(d[0]) {
			add(ruleSOQLInLoop, d[0], "Dynamic SOQL via Database.query", "", "")
		}
	}
	for _, d := range f.dml {
		if f.inLoop(d) {
			add(ruleDMLInLoop, d, "", "", "")
		}
	}
	for _, m := range reGGD.FindAllStringIndex(f.clean, -1) {
		add(ruleGGD, m[0], "", "", "")
	}

	// One-hop: a call inside a loop to a method in this file that issues SOQL/DML.
	soqlAll := append([]span{}, f.soql...)
	for _, d := range dynQueries {
		soqlAll = append(soqlAll, span{d[0], d[1]})
	}
	for _, m := range f.methods {
		mSOQL, mDML := hasOp(m.body, soqlAll, f.dml)
		if !mSOQL && !mDML {
			continue
		}
		reCall := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(m.name) + `\s*\(`)
		for _, call := range reCall.FindAllStringIndex(f.clean, -1) {
			if m.body.contains(call[0]) || !f.inLoop(call[0]) || f.isDeclaration(call[0]) {
				continue
			}
			if mSOQL {
				add(ruleSOQLInLoopOneHop, call[0], "Calls "+m.name+"(), which issues SOQL", "", "")
			}
			if mDML {
				add(ruleDMLInLoopOneHop, call[0], "Calls "+m.name+"(), which performs DML", "", "")
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Rule.Code < out[j].Rule.Code
	})
	return out
}

// isDeclaration reports whether pos is the name in a method declaration.
func (f *apexFile) isDeclaration(pos int) bool {
	for _, m := range f.methods {
		if m.namePos == pos {
			return true
		}
	}
	return false
}

// unusedFields returns selected field names that the file never reads (as
// `.Field`) outside the query itself. Id, subqueries, aggregates, and TYPEOF
// are ignored because their usage cannot be tracked syntactically.
func (f *apexFile) unusedFields(selectList string, q span) []string {
	outside := f.clean[:q.start] + strings.Repeat(" ", q.end-q.start) + f.clean[q.end:]
	var unused []string
	for _, raw := range splitTopLevel(selectList) {
		field := strings.TrimSpace(raw)
		if field == "" || strings.ContainsAny(field, "( ") || strings.HasPrefix(strings.ToLower(field), "typeof") {
			continue
		}
		name := field
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		if strings.EqualFold(name, "Id") {
			continue
		}
		re := regexp.MustCompile(`(?i)\.\s*` + regexp.QuoteMeta(name) + `\b`)
		if !re.MatchString(outside) {
			unused = append(unused, field)
		}
	}
	return unused
}

// splitTopLevel splits a select list on commas at parenthesis depth zero.
func splitTopLevel(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}
