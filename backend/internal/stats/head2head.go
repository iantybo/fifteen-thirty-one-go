package stats

import (
	"sort"
)

// ComputeHeadToHead computes head-to-head stats between two players.
func ComputeHeadToHead(games []Game, playerA, playerB string) HeadToHead {
	result := HeadToHead{PlayerA: playerA, PlayerB: playerB}
	var marginSum float64
	for _, game := range games {
		if !isMatchup(game, playerA, playerB) {
			continue
		}
		result.Games++
		if game.EndedAt.After(result.LastMet) {
			result.LastMet = game.EndedAt
		}
		fromA := game.PlayerID == playerA
		var resForA GameResult
		var marginForA int
		if fromA {
			resForA = game.Result
			marginForA = game.PlayerScore - game.OppScore
		} else {
			resForA = flip(game.Result)
			marginForA = game.OppScore - game.PlayerScore
		}
		switch resForA {
		case ResultWin:
			result.AWins++
		case ResultLoss:
			result.BWins++
		case ResultDraw:
			result.Draws++
		}
		marginSum += float64(marginForA)
	}
	if result.Games > 0 {
		result.AvgMargin = marginSum / float64(result.Games)
	}
	return result
}

func isMatchup(game Game, playerA, playerB string) bool {
	return (game.PlayerID == playerA && game.OpponentID == playerB) ||
		(game.PlayerID == playerB && game.OpponentID == playerA)
}

func flip(result GameResult) GameResult {
	switch result {
	case ResultWin:
		return ResultLoss
	case ResultLoss:
		return ResultWin
	default:
		return result
	}
}

// HeadToHeadMatrix builds an all-pairs matrix for the supplied players.
func HeadToHeadMatrix(games []Game, players []string) map[string]map[string]HeadToHead {
	out := make(map[string]map[string]HeadToHead, len(players))
	for _, playerA := range players {
		row := make(map[string]HeadToHead, len(players))
		for _, playerB := range players {
			if playerA == playerB {
				continue
			}
			row[playerB] = ComputeHeadToHead(games, playerA, playerB)
		}
		out[playerA] = row
	}
	return out
}

// RivalryRanking returns opponents sorted by total games played.
func RivalryRanking(games []Game, playerA string) []HeadToHead {
	counts := make(map[string]int)
	for _, game := range games {
		switch {
		case game.PlayerID == playerA:
			counts[game.OpponentID]++
		case game.OpponentID == playerA:
			counts[game.PlayerID]++
		}
	}
	opps := make([]string, 0, len(counts))
	for oppID := range counts {
		opps = append(opps, oppID)
	}
	sort.Slice(opps, func(ii, jj int) bool {
		if counts[opps[ii]] != counts[opps[jj]] {
			return counts[opps[ii]] > counts[opps[jj]]
		}
		return opps[ii] < opps[jj]
	})
	out := make([]HeadToHead, 0, len(opps))
	for _, opp := range opps {
		out = append(out, ComputeHeadToHead(games, playerA, opp))
	}
	return out
}

// DominanceScore returns a 0..1 score of how dominant A is over B.
func (h2h HeadToHead) DominanceScore() float64 {
	if h2h.Games == 0 {
		return 0
	}
	wins := float64(h2h.AWins) + 0.5*float64(h2h.Draws)
	return wins / float64(h2h.Games)
}

// IsRival reports whether two players have played at least the threshold games.
func (h2h HeadToHead) IsRival(threshold int) bool {
	return h2h.Games >= threshold
}

// SwingFactor measures how lopsided a head-to-head is (1.0 = totally lopsided, 0.0 = even).
func (h2h HeadToHead) SwingFactor() float64 {
	if h2h.Games == 0 {
		return 0
	}
	diff := h2h.AWins - h2h.BWins
	if diff < 0 {
		diff = -diff
	}
	return float64(diff) / float64(h2h.Games)
}
