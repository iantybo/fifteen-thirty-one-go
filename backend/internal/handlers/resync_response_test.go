package handlers

import (
	"reflect"
	"testing"
)

func TestEnsureNonNilIDs(t *testing.T) {
	t.Parallel()

	if got := ensureNonNilIDs(nil); got == nil || len(got) != 0 {
		t.Errorf("nil input should become empty non-nil slice, got %#v", got)
	}
	in := []string{"a", "b"}
	if got := ensureNonNilIDs(in); !reflect.DeepEqual(got, in) {
		t.Errorf("non-nil input should pass through, got %#v", got)
	}
}

func TestDedupeIDs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty", []string{}, []string{}},
		{"no dupes", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"adjacent dupes", []string{"a", "a", "b"}, []string{"a", "b"}},
		{"scattered dupes preserve first-seen order", []string{"b", "a", "b", "c", "a"}, []string{"b", "a", "c"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := dedupeIDs(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("dedupeIDs(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestPartitionPending(t *testing.T) {
	t.Parallel()

	pending := []resyncPendingAction{
		{ClientID: "c:1:9:1", Kind: "play_card"},
		{ClientID: "", Kind: "go"},
		{ClientID: "garbage", Kind: "go"},
		{ClientID: "c:1:9:2", Kind: "discard"},
	}
	valid, invalid := partitionPending(pending)

	if len(valid) != 2 {
		t.Errorf("expected 2 valid, got %d (%v)", len(valid), valid)
	}
	if len(invalid) != 2 {
		t.Errorf("expected 2 invalid, got %d (%v)", len(invalid), invalid)
	}
	if valid[0].ClientID != "c:1:9:1" || valid[1].ClientID != "c:1:9:2" {
		t.Errorf("valid entries in wrong order: %v", valid)
	}
}

func TestPartitionPending_AllValid_InvalidIsNonNil(t *testing.T) {
	t.Parallel()

	valid, invalid := partitionPending([]resyncPendingAction{{ClientID: "c:1:9:1", Kind: "go"}})
	if len(valid) != 1 {
		t.Errorf("expected 1 valid, got %d", len(valid))
	}
	if invalid == nil {
		t.Error("invalid slice should be non-nil even when empty")
	}
}
