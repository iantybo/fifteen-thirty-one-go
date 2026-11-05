package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed seed/volumes.json seed/hotspots.json
var seedFS embed.FS

// environments lists the simulated orgs in promotion order.
var environments = []string{"dev", "uat", "prod"}

// objectVolume is the seeded row count for one sObject in one environment.
type objectVolume struct {
	Rows int64 `json:"rows"`
}

// envData is one simulated org: a label plus per-object volumes.
type envData struct {
	Label   string                  `json:"label"`
	Objects map[string]objectVolume `json:"objects"`
}

// hotspot is one seeded runtime telemetry row for a class method.
type hotspot struct {
	Method            string `json:"method"`
	AvgCPUMs          int    `json:"avg_cpu_ms"`
	P95CPUMs          int    `json:"p95_cpu_ms"`
	AvgHeapKB         int    `json:"avg_heap_kb"`
	P95SOQLQueries    int    `json:"p95_soql_queries"`
	P95DMLStatements  int    `json:"p95_dml_statements"`
	InvocationsPerDay int    `json:"invocations_per_day"`
	Exceptions7d      int    `json:"exceptions_7d"`
	Note              string `json:"note,omitempty"`
}

// seedData is the full immutable simulated dataset, loaded once at startup.
type seedData struct {
	Environments map[string]envData              `json:"environments"`
	Indexed      map[string]map[string]float64   `json:"indexed"`
	Notes        map[string]string               `json:"notes"`
	Hotspots     map[string]map[string][]hotspot `json:"-"`
	objectNames  map[string]string               // lower-case name -> canonical name
	hotspotNames map[string]string               // lower-case class -> canonical class
}

// loadSeed parses the embedded seed files, returning a descriptive error when
// either file is missing or malformed.
func loadSeed() (*seedData, error) {
	raw, err := seedFS.ReadFile("seed/volumes.json")
	if err != nil {
		return nil, fmt.Errorf("read seed/volumes.json: %w", err)
	}
	var s seedData
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse seed/volumes.json: %w", err)
	}
	for _, env := range environments {
		if _, ok := s.Environments[env]; !ok {
			return nil, fmt.Errorf("seed/volumes.json: missing environment %q", env)
		}
	}

	raw, err = seedFS.ReadFile("seed/hotspots.json")
	if err != nil {
		return nil, fmt.Errorf("read seed/hotspots.json: %w", err)
	}
	var hs struct {
		Classes map[string]map[string][]hotspot `json:"classes"`
	}
	if err := json.Unmarshal(raw, &hs); err != nil {
		return nil, fmt.Errorf("parse seed/hotspots.json: %w", err)
	}
	s.Hotspots = hs.Classes

	s.objectNames = make(map[string]string)
	for _, env := range s.Environments {
		for name := range env.Objects {
			s.objectNames[strings.ToLower(name)] = name
		}
	}
	s.hotspotNames = make(map[string]string)
	for name := range s.Hotspots {
		s.hotspotNames[strings.ToLower(name)] = name
	}
	return &s, nil
}

// validEnv normalizes env (defaulting to uat) and rejects unknown values.
func validEnv(env string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(env))
	if e == "" {
		return "uat", nil
	}
	for _, known := range environments {
		if e == known {
			return e, nil
		}
	}
	return "", fmt.Errorf("unknown environment %q; expected one of %s", env, strings.Join(environments, ", "))
}

// canonicalObject resolves an sObject name case-insensitively (Apex is
// case-insensitive). ok is false when the object is not in the seed.
func (s *seedData) canonicalObject(name string) (string, bool) {
	c, ok := s.objectNames[strings.ToLower(strings.TrimSpace(name))]
	return c, ok
}

// rows returns the seeded row count for object in env.
func (s *seedData) rows(env, object string) (int64, bool) {
	c, ok := s.canonicalObject(object)
	if !ok {
		return 0, false
	}
	v, ok := s.Environments[env].Objects[c]
	return v.Rows, ok
}

// indexedFraction returns the selectivity fraction for an indexed field and
// whether the field is indexed at all.
func (s *seedData) indexedFraction(object, field string) (float64, bool) {
	c, ok := s.canonicalObject(object)
	if !ok {
		return 0, false
	}
	for f, frac := range s.Indexed[c] {
		if strings.EqualFold(f, field) {
			return frac, true
		}
	}
	return 0, false
}

// sortedObjects returns the canonical object names in a stable order.
func (s *seedData) sortedObjects() []string {
	out := make([]string, 0, len(s.objectNames))
	for _, c := range s.objectNames {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
