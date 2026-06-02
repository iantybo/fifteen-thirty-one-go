package stats

import (
	"sort"
)

// ComputeHeadToHead computes head-to-head stats between two players.
func ComputeHeadToHead(games []Game, playerA, playerB string) HeadToHead {
	h := HeadToHead{PlayerA: playerA, PlayerB: playerB}
	var marginSum float64
	for _, g := range games {
		if !isMatchup(g, playerA, playerB) {
			continue
		}
		h.Games++
		if g.EndedAt.After(h.LastMet) {
			h.LastMet = g.EndedAt
		}
		fromA := g.PlayerID == playerA
		var resForA GameResult
		var marginForA int
		if fromA {
			resForA = g.Result
			marginForA = g.PlayerScore - g.OppScore
		} else {
			resForA = flip(g.Result)
			marginForA = g.OppScore - g.PlayerScore
		}
		switch resForA {
		case ResultWin:
			h.AWins++
		case ResultLoss:
			h.BWins++
		case ResultDraw:
			h.Draws++
		}
		marginSum += float64(marginForA)
	}
	if h.Games > 0 {
		h.AvgMargin = marginSum / float64(h.Games)
	}
	return h
}

func isMatchup(g Game, a, b string) bool {
	return (g.PlayerID == a && g.OpponentID == b) ||
		(g.PlayerID == b && g.OpponentID == a)
}

func flip(r GameResult) GameResult {
	switch r {
	case ResultWin:
		return ResultLoss
	case ResultLoss:
		return ResultWin
	default:
		return r
	}
}

// HeadToHeadMatrix builds an all-pairs matrix for the supplied players.
func HeadToHeadMatrix(games []Game, players []string) map[string]map[string]HeadToHead {
	out := make(map[string]map[string]HeadToHead, len(players))
	for _, a := range players {
		row := make(map[string]HeadToHead, len(players))
		for _, b := range players {
			if a == b {
				continue
			}
			row[b] = ComputeHeadToHead(games, a, b)
		}
		out[a] = row
	}
	return out
}

// RivalryRanking returns opponents sorted by total games played.
func RivalryRanking(games []Game, playerA string) []HeadToHead {
	counts := make(map[string]int)
	for _, g := range games {
		switch {
		case g.PlayerID == playerA:
			counts[g.OpponentID]++
		case g.OpponentID == playerA:
			counts[g.PlayerID]++
		}
	}
	opps := make([]string, 0, len(counts))
	for k := range counts {
		opps = append(opps, k)
	}
	sort.Slice(opps, func(i, j int) bool {
		if counts[opps[i]] != counts[opps[j]] {
			return counts[opps[i]] > counts[opps[j]]
		}
		return opps[i] < opps[j]
	})
	out := make([]HeadToHead, 0, len(opps))
	for _, o := range opps {
		out = append(out, ComputeHeadToHead(games, playerA, o))
	}
	return out
}

// DominanceScore returns a 0..1 score of how dominant A is over B.
func (h HeadToHead) DominanceScore() float64 {
	if h.Games == 0 {
		return 0
	}
	wins := float64(h.AWins) + 0.5*float64(h.Draws)
	return wins / float64(h.Games)
}

// IsRival reports whether two players have played at least the threshold games.
func (h HeadToHead) IsRival(threshold int) bool {
	return h.Games >= threshold
}

// SwingFactor measures how lopsided a head-to-head is (1.0 = totally lopsided, 0.0 = even).
func (h HeadToHead) SwingFactor() float64 {
	if h.Games == 0 {
		return 0
	}
	diff := h.AWins - h.BWins
	if diff < 0 {
		diff = -diff
	}
	return float64(diff) / float64(h.Games)
}
