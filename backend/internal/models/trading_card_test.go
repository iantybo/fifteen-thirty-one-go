package models

import "testing"

func TestCanClaimAnyReward(t *testing.T) {
	rewards := []CardRewardCondition{
		{Current: 0, Target: 10},
		{Current: 15, Target: 12},
	}

	if !CanClaimAnyReward(rewards) {
		t.Fatalf("expected OR-based reward eligibility")
	}
}

func TestGetUserCardProgressUsesHighestCompletion(t *testing.T) {
	rewards := []CardRewardCondition{
		{Current: 0, Target: 10},   // 0%
		{Current: 6, Target: 8},    // 75%
		{Current: 40, Target: 100}, // 40%
	}

	progress := GetUserCardProgress(rewards)
	if progress.RewardIndex != 1 {
		t.Fatalf("expected reward index 1, got %d", progress.RewardIndex)
	}
	if progress.Percent != 75 {
		t.Fatalf("expected 75 percent, got %v", progress.Percent)
	}
}

func TestGetUserCardProgressEmpty(t *testing.T) {
	progress := GetUserCardProgress(nil)
	if progress.RewardIndex != -1 {
		t.Fatalf("expected -1 index for empty rewards, got %d", progress.RewardIndex)
	}
	if progress.Percent != 0 {
		t.Fatalf("expected 0 percent for empty rewards, got %v", progress.Percent)
	}
}
