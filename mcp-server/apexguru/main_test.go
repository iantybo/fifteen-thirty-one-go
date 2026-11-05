package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustSeed(t *testing.T) *seedData {
	t.Helper()
	s, err := loadSeed()
	if err != nil {
		t.Fatalf("loadSeed: %v", err)
	}
	return s
}

// rulesOf returns the rule codes in vs, keyed by line.
func rulesOf(vs []Violation) map[string][]int {
	out := map[string][]int{}
	for _, v := range vs {
		out[v.Rule.Code] = append(out[v.Rule.Code], v.Line)
	}
	return out
}

func TestDetectRules(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want map[string][]int // rule code -> lines
	}{
		{
			name: "soql in braced for loop",
			src:  "public class A {\n  void m(List<Id> ids) {\n    for (Id i : ids) {\n      Lead l = [SELECT Id FROM Lead WHERE Id = :i];\n    }\n  }\n}",
			want: map[string][]int{"SOQL_IN_LOOP": {4}},
		},
		{
			name: "soql in for header is not in loop",
			src:  "public class A {\n  void m() {\n    for (Lead l : [SELECT Id FROM Lead WHERE OwnerId = :UserInfo.getUserId()]) {\n      l.Id = null;\n    }\n  }\n}",
			want: map[string][]int{},
		},
		{
			name: "dml in single-statement while body",
			src:  "public class A {\n  void m(List<Lead> ls) {\n    Integer i = 0;\n    while (i < 3) insert ls[i++];\n  }\n}",
			want: map[string][]int{"DML_IN_LOOP": {4}},
		},
		{
			name: "dml after loop is fine",
			src:  "public class A {\n  void m(List<Lead> ls) {\n    for (Lead l : ls) {\n      l.Status = 'Open';\n    }\n    update ls;\n  }\n}",
			want: map[string][]int{},
		},
		{
			name: "database dml in do-while",
			src:  "public class A {\n  void m(List<Lead> ls) {\n    do {\n      Database.insert(ls);\n    } while (false);\n  }\n}",
			want: map[string][]int{"DML_IN_LOOP": {4}},
		},
		{
			name: "one hop through helper",
			src:  "public class A {\n  void m(List<Id> ids) {\n    for (Id i : ids) {\n      load(i);\n    }\n  }\n  Lead load(Id i) {\n    return [SELECT Id FROM Lead WHERE Id = :i];\n  }\n}",
			want: map[string][]int{"SOQL_IN_LOOP_1HOP": {4}},
		},
		{
			name: "global describe",
			src:  "public class A {\n  void m() {\n    Schema.getGlobalDescribe();\n  }\n}",
			want: map[string][]int{"GGD": {3}},
		},
		{
			name: "no where no limit and unused fields",
			src:  "public class A {\n  void m() {\n    List<User> us = [SELECT Id, Email, Phone FROM User];\n    System.debug(us[0].Email);\n  }\n}",
			want: map[string][]int{"SOQL_NO_WHERE_OR_LIMIT": {3}, "SOQL_UNUSED_FIELDS": {3}},
		},
		{
			name: "keywords in comments and strings are ignored",
			src:  "public class A {\n  // for (x) { insert y; }\n  void m() {\n    String s = 'for (a) { [SELECT Id FROM User] }';\n  }\n}",
			want: map[string][]int{},
		},
		{
			name: "isolated test class skips unbounded-query rule",
			src:  "@IsTest\nprivate class ATest {\n  @IsTest static void t() {\n    Integer n = [SELECT COUNT() FROM Lead];\n  }\n}",
			want: map[string][]int{},
		},
		{
			name: "SeeAllData test class is not isolated",
			src:  "@IsTest(SeeAllData=true)\nprivate class ATest {\n  @IsTest static void t() {\n    Integer n = [SELECT COUNT() FROM Lead];\n  }\n}",
			want: map[string][]int{"SOQL_NO_WHERE_OR_LIMIT": {4}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rulesOf(parseApex("A.cls", tt.src).detect())
			if len(got) != len(tt.want) {
				t.Fatalf("rules = %v, want %v", got, tt.want)
			}
			for code, lines := range tt.want {
				if fmt.Sprint(got[code]) != fmt.Sprint(lines) {
					t.Errorf("%s lines = %v, want %v", code, got[code], lines)
				}
			}
		})
	}
}

func TestDemoFixtureFlagsEveryLayer(t *testing.T) {
	s := mustSeed(t)
	src, err := os.ReadFile("testdata/SignupBackfill.apex.txt")
	if err != nil {
		t.Fatal(err)
	}
	vs := parseApex("SignupBackfill.cls", string(src)).detect()
	got := rulesOf(vs)
	for _, code := range []string{"GGD", "SOQL_NO_WHERE_OR_LIMIT", "SOQL_UNUSED_FIELDS", "SOQL_IN_LOOP", "DML_IN_LOOP", "DML_IN_LOOP_1HOP"} {
		if len(got[code]) == 0 {
			t.Errorf("expected %s in demo fixture; got %v", code, got)
		}
	}

	out, err := s.scan(scanArgs{Files: []scanFile{{Path: "SignupBackfill.cls", Content: string(src)}}, Environment: "uat"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Production insights (simulated uat)", "412,000", "Too many query rows", "Simulated ApexGuru"} {
		if !strings.Contains(out, want) {
			t.Errorf("uat scan output missing %q:\n%s", want, out)
		}
	}
	dev, err := s.scan(scanArgs{Files: []scanFile{{Path: "SignupBackfill.cls", Content: string(src)}}, Environment: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dev, "Static only") || strings.Contains(dev, "Too many query rows") {
		t.Errorf("dev scan should be static-only without volume failures:\n%s", dev)
	}
}

// TestBaselineSalesforceModuleIsClean guards the POV baseline: the committed
// sfdx module must scan clean at prod volumes so the demo PR is the only place
// violations appear.
func TestBaselineSalesforceModuleIsClean(t *testing.T) {
	s := mustSeed(t)
	root := filepath.Join("..", "..", "salesforce", "force-app")
	var files []scanFile
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ext := filepath.Ext(p); !d.IsDir() && (ext == ".cls" || ext == ".trigger") {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files = append(files, scanFile{Path: p, Content: string(b)})
		}
		return nil
	})
	if err != nil {
		t.Skipf("salesforce module not present: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no Apex files found")
	}
	for _, env := range environments {
		out, err := s.scan(scanArgs{Files: files, Environment: env, Top: 50})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "**Found 0 performance violations**") {
			t.Errorf("baseline module has findings in %s:\n%s", env, out)
		}
	}
}

func TestVolumeOnlyNonSelectiveQuery(t *testing.T) {
	s := mustSeed(t)
	src := "public with sharing class ACME_LeadTriggerHandler {\n  public static void afterInsert(List<Lead> ls) {\n    List<Lead> open = [SELECT Id FROM Lead WHERE LeadSource = 'x' AND IsConverted = false];\n  }\n}"
	files := []scanFile{{Path: "ACME_LeadTriggerHandler.cls", Content: src}}
	if got := parseApex(files[0].Path, src).detect(); len(got) != 0 {
		t.Fatalf("static rules should pass a query with a WHERE clause, got %v", rulesOf(got))
	}
	uat, err := s.scan(scanArgs{Files: files, Environment: "uat"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SoqlNonSelectiveAtVolume", "Non-selective query against large object type", "61/100"} {
		if !strings.Contains(uat, want) {
			t.Errorf("uat scan missing %q:\n%s", want, uat)
		}
	}
	dev, err := s.scan(scanArgs{Files: files, Environment: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dev, "SoqlNonSelectiveAtVolume") {
		t.Errorf("dev scan must not flag volume-only issues:\n%s", dev)
	}
}

func TestExplainSOQL(t *testing.T) {
	s := mustSeed(t)
	tests := []struct {
		name, query, env, ctx string
		wantPass              bool
		wantPlan              string
	}{
		{"unbounded user passes dev", "SELECT Id, Email FROM User", "dev", "sync", true, "TableScan"},
		{"unbounded user fails uat", "SELECT Id, Email FROM User", "uat", "sync", false, "TableScan"},
		{"limit caps rows", "SELECT Id FROM User LIMIT 200", "prod", "sync", true, "TableScan"},
		{"indexed equality is selective", "SELECT Id FROM Lead WHERE Email = :e", "prod", "trigger", true, "Index"},
		{"non-indexed filter in trigger fails", "SELECT Id FROM Lead WHERE Status = 'Open'", "uat", "trigger", false, "TableScan"},
		{"leading wildcard is not selective", "SELECT Id FROM Lead WHERE Email LIKE '%@example.com' LIMIT 10", "uat", "trigger", false, "TableScan"},
		{"negative operator is not selective", "SELECT Id FROM Lead WHERE Email != null LIMIT 10", "uat", "trigger", false, "TableScan"},
		{"or with non-indexed branch", "SELECT Id FROM Lead WHERE Email = :e OR Status = 'Open' LIMIT 10", "uat", "trigger", false, "TableScan"},
		{"subquery from is ignored", "SELECT Id, (SELECT Id FROM CampaignMembers) FROM Campaign WHERE Name = :n LIMIT 1", "prod", "trigger", true, "Index"},
		{"batch context allows large scans", "SELECT Id FROM Lead", "prod", "batch", true, "TableScan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := s.explainSOQL(tt.query, tt.env, tt.ctx)
			if err != nil {
				t.Fatal(err)
			}
			if p.Pass() != tt.wantPass || p.Plan != tt.wantPlan {
				t.Errorf("pass=%v plan=%s failures=%v; want pass=%v plan=%s", p.Pass(), p.Plan, p.Failures, tt.wantPass, tt.wantPlan)
			}
		})
	}
	if _, err := s.explainSOQL("UPDATE Lead", "uat", "sync"); err == nil {
		t.Error("expected error for non-SELECT")
	}
}

func TestToolInputValidation(t *testing.T) {
	s := mustSeed(t)
	if _, err := s.scan(scanArgs{}); err == nil {
		t.Error("scan with no files should fail")
	}
	if _, err := s.scan(scanArgs{Files: []scanFile{{Path: "A.cls", Content: "x"}}, Environment: "staging"}); err == nil {
		t.Error("unknown environment should fail")
	}
	big := strings.Repeat("a", maxScanTotalBytes+1)
	if _, err := s.scan(scanArgs{Files: []scanFile{{Path: "A.cls", Content: big}}}); err == nil {
		t.Error("oversized scan should fail")
	}
	if _, err := s.explain(explainArgs{Query: "SELECT Id FROM Lead", Context: "future"}); err == nil {
		t.Error("unknown context should fail")
	}
	if _, err := s.volumes(volumesArgs{Object: "Opportunityy"}); err == nil {
		t.Error("unknown object should fail")
	}
	out, err := s.hotspots(hotspotArgs{ClassName: "acme_leadtriggerhandler"})
	if err != nil || !strings.Contains(out, "61/100") {
		t.Errorf("hotspots lookup should be case-insensitive and include p95 SOQL; err=%v out=%s", err, out)
	}
	out, err = s.hotspots(hotspotArgs{ClassName: "SignupBackfill"})
	if err != nil || !strings.Contains(out, "No runtime telemetry") {
		t.Errorf("unknown class should report no telemetry; err=%v out=%s", err, out)
	}
}

func TestRequireAPIKey(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := requireAPIKey("secret", ok)
	tests := []struct {
		name   string
		header map[string]string
		want   int
	}{
		{"missing", nil, http.StatusUnauthorized},
		{"wrong bearer", map[string]string{"Authorization": "Bearer nope"}, http.StatusUnauthorized},
		{"non-bearer scheme", map[string]string{"Authorization": "secret"}, http.StatusUnauthorized},
		{"bearer", map[string]string{"Authorization": "Bearer secret"}, http.StatusNoContent},
		{"x-api-key", map[string]string{"X-API-Key": "secret"}, http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			for k, v := range tt.header {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Errorf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}

func TestHealthzIsOpen(t *testing.T) {
	mux := newMux(newServer(mustSeed(t)), "secret", splitList(defaultAllowedHostSuffixes))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Errorf("healthz = %d, want 200", w.Code)
	}
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Host = "abc.ngrok-free.dev"
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("/mcp without key = %d, want 401", w.Code)
	}

	r = httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Host = "attacker.test"
	r.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("/mcp from disallowed host = %d, want 403", w.Code)
	}
}

func TestHostAllowed(t *testing.T) {
	allowed := splitList(defaultAllowedHostSuffixes + ",mcp.example.com")
	tests := []struct {
		host string
		want bool
	}{
		{"localhost:8766", true},
		{"127.0.0.1:8766", true},
		{"[::1]:8766", true},
		{"abc-123.ngrok-free.dev", true},
		{"abc.ngrok-free.app", true},
		{"mcp.example.com", true},
		{"ngrok-free.dev", false},
		{"evil.example.com", false},
		{"abc.ngrok-free.dev.evil.com", false},
		{"attacker.test", false},
	}
	for _, tt := range tests {
		if got := hostAllowed(tt.host, allowed); got != tt.want {
			t.Errorf("hostAllowed(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}
