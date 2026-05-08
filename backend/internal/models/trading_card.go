package models

import "math"

// CardRewardCondition captures one claimable condition row for a trading card.
type CardRewardCondition struct {
	Current int64 `json:"current"`
	Target  int64 `json:"target"`
}

// CardProgress summarizes display progress while preserving claim eligibility semantics.
type CardProgress struct {
	Eligible     bool    `json:"eligible"`
	Percent      float64 `json:"percent"`
	RewardIndex  int     `json:"reward_index"`
	CurrentValue int64   `json:"current_value"`
	TargetValue  int64   `json:"target_value"`
}

// CanClaimAnyReward evaluates reward conditions with OR semantics.
func CanClaimAnyReward(rewards []CardRewardCondition) bool {
	for _, reward := range rewards {
		if reward.Target > 0 && reward.Current >= reward.Target {
			return true
		}
	}
	return false
}

// GetUserCardProgress returns progress based on the most complete reward condition.
// This keeps progress display aligned with OR-based claim eligibility.
func GetUserCardProgress(rewards []CardRewardCondition) CardProgress {
	if len(rewards) == 0 {
		return CardProgress{
			Eligible:    false,
			Percent:     0,
			RewardIndex: -1,
		}
	}

	bestIndex := 0
	bestPercent := completionPercent(rewards[0].Current, rewards[0].Target)
	for idx := 1; idx < len(rewards); idx++ {
		pct := completionPercent(rewards[idx].Current, rewards[idx].Target)
		if pct > bestPercent {
			bestPercent = pct
			bestIndex = idx
		}
	}

	best := rewards[bestIndex]
	return CardProgress{
		Eligible:     CanClaimAnyReward(rewards),
		Percent:      bestPercent,
		RewardIndex:  bestIndex,
		CurrentValue: best.Current,
		TargetValue:  best.Target,
	}
}

func completionPercent(current, target int64) float64 {
	if target <= 0 || current <= 0 {
		return 0
	}
	raw := (float64(current) / float64(target)) * 100
	return math.Min(100, raw)
}
