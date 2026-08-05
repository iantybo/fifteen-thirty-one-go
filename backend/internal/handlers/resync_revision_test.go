package handlers

import "testing"

// revisionIsNewer and revisionIsStale are pure comparisons (no DB). Together
// with equality they partition every (incoming, current) pair into exactly one
// case; the tests assert both the individual truth tables and that complement
// property.

func TestRevisionIsNewer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		incoming int64
		current  int64
		want     bool
	}{
		{"incoming greater", 5, 3, true},
		{"incoming equal", 4, 4, false},
		{"incoming less", 2, 3, false},
		{"both zero", 0, 0, false},
		{"negative incoming", -1, 0, false},
		{"large gap", 1000, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := revisionIsNewer(tc.incoming, tc.current); got != tc.want {
				t.Errorf("revisionIsNewer(%d, %d) = %v, want %v", tc.incoming, tc.current, got, tc.want)
			}
		})
	}
}

func TestRevisionIsStale(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		incoming int64
		current  int64
		want     bool
	}{
		{"incoming greater", 5, 3, false},
		{"incoming equal", 4, 4, false},
		{"incoming less", 2, 3, true},
		{"both zero", 0, 0, false},
		{"negative incoming", -1, 0, true},
		{"large gap", 1, 1000, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := revisionIsStale(tc.incoming, tc.current); got != tc.want {
				t.Errorf("revisionIsStale(%d, %d) = %v, want %v", tc.incoming, tc.current, got, tc.want)
			}
		})
	}
}

// TestRevisionPartition asserts that newer, stale, and equal are mutually
// exclusive and exhaustive: exactly one holds for any pair.
func TestRevisionPartition(t *testing.T) {
	t.Parallel()

	pairs := [][2]int64{
		{0, 0}, {1, 0}, {0, 1}, {3, 3}, {5, 2}, {2, 5}, {-1, 1}, {1, -1},
	}
	for _, p := range pairs {
		incoming, current := p[0], p[1]
		newer := revisionIsNewer(incoming, current)
		stale := revisionIsStale(incoming, current)
		equal := incoming == current

		count := 0
		for _, b := range []bool{newer, stale, equal} {
			if b {
				count++
			}
		}
		if count != 1 {
			t.Errorf("(%d, %d): expected exactly one of newer/stale/equal, got newer=%v stale=%v equal=%v",
				incoming, current, newer, stale, equal)
		}
		// newer and stale must be strict complements when not equal.
		if !equal && newer == stale {
			t.Errorf("(%d, %d): newer and stale should be complements when unequal, got newer=%v stale=%v",
				incoming, current, newer, stale)
		}
	}
}
