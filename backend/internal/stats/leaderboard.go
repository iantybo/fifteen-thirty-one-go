package stats

import (
	"errors"
	"sort"
	"time"
)

// LeaderboardOptions controls leaderboard construction.
type LeaderboardOptions struct {
	Mode        GameMode
	MinGames    int
	Limit       int
	ActiveOnly  bool
	ActiveSince time.Time
}

// DefaultLeaderboardOptions returns standard defaults.
func DefaultLeaderboardOptions() LeaderboardOptions {
	return LeaderboardOptions{
		Mode:     ModeRanked,
		MinGames: 5,
		Limit:    100,
	}
}

// BuildLeaderboard constructs a leaderboard from summaries and rating engine.
func BuildLeaderboard(summaries map[string]PlayerSummary, engine *Engine, opts LeaderboardOptions, now time.Time) (Leaderboard, error) {
	if summaries == nil {
		return Leaderboard{}, errors.New("nil summaries")
	}
	if engine == nil {
		return Leaderboard{}, errors.New("nil engine")
	}

	entries := make([]LeaderboardEntry, 0, len(summaries))
	for playerID, summary := range summaries {
		if opts.MinGames > 0 && summary.Games < opts.MinGames {
			continue
		}
		if opts.ActiveOnly && !opts.ActiveSince.IsZero() && summary.LastPlayed.Before(opts.ActiveSince) {
			continue
		}
		modeStats := summary.ModeBreakdown[opts.Mode]
		winRate := summary.WinRate
		games := summary.Games
		if opts.Mode != ModeUnknown && modeStats.Games > 0 {
			winRate = modeStats.WinRate
			games = modeStats.Games
		}
		rating := engine.Get(playerID)
		entries = append(entries, LeaderboardEntry{
			PlayerID: playerID,
			Rating:   rating,
			Games:    games,
			WinRate:  winRate,
			LastSeen: summary.LastPlayed,
			Trend:    summary.RatingDelta30Day,
		})
	}

	sort.Slice(entries, func(ii, jj int) bool {
		if entries[ii].Rating != entries[jj].Rating {
			return entries[ii].Rating > entries[jj].Rating
		}
		if entries[ii].Games != entries[jj].Games {
			return entries[ii].Games > entries[jj].Games
		}
		return entries[ii].PlayerID < entries[jj].PlayerID
	})

	for idx := range entries {
		entries[idx].Rank = idx + 1
	}
	if opts.Limit > 0 && len(entries) > opts.Limit {
		entries = entries[:opts.Limit]
	}

	return Leaderboard{
		Mode:      opts.Mode,
		Entries:   entries,
		UpdatedAt: now,
	}, nil
}

// FindEntry finds a player's leaderboard entry by ID.
func (lb Leaderboard) FindEntry(playerID string) (LeaderboardEntry, bool) {
	for _, entry := range lb.Entries {
		if entry.PlayerID == playerID {
			return entry, true
		}
	}
	return LeaderboardEntry{}, false
}

// Top returns the first count entries.
func (lb Leaderboard) Top(count int) []LeaderboardEntry {
	if count <= 0 || count >= len(lb.Entries) {
		return lb.Entries
	}
	return lb.Entries[:count]
}

// Around returns entries surrounding the rank of the specified player.
func (lb Leaderboard) Around(playerID string, span int) []LeaderboardEntry {
	if span < 0 {
		span = 0
	}
	for idx, entry := range lb.Entries {
		if entry.PlayerID == playerID {
			lo := idx - span
			if lo < 0 {
				lo = 0
			}
			hi := idx + span + 1
			if hi > len(lb.Entries) {
				hi = len(lb.Entries)
			}
			return lb.Entries[lo:hi]
		}
	}
	return nil
}

// MultiLeaderboards builds one leaderboard per mode.
func MultiLeaderboards(summaries map[string]PlayerSummary, engine *Engine, modes []GameMode, base LeaderboardOptions, now time.Time) (map[GameMode]Leaderboard, error) {
	out := make(map[GameMode]Leaderboard, len(modes))
	for _, mode := range modes {
		opts := base
		opts.Mode = mode
		lb, err := BuildLeaderboard(summaries, engine, opts, now)
		if err != nil {
			return nil, err
		}
		out[mode] = lb
	}
	return out, nil
}

// Movement reports rank change vs a previous snapshot.
type Movement struct {
	PlayerID string
	OldRank  int
	NewRank  int
	Change   int
}

// CompareLeaderboards diffs two leaderboards by player rank.
func CompareLeaderboards(prev, curr Leaderboard) []Movement {
	prevRanks := make(map[string]int, len(prev.Entries))
	for _, entry := range prev.Entries {
		prevRanks[entry.PlayerID] = entry.Rank
	}
	var out []Movement
	for _, entry := range curr.Entries {
		oldRank, ok := prevRanks[entry.PlayerID]
		if !ok {
			out = append(out, Movement{PlayerID: entry.PlayerID, OldRank: 0, NewRank: entry.Rank, Change: 0})
			continue
		}
		out = append(out, Movement{
			PlayerID: entry.PlayerID,
			OldRank:  oldRank,
			NewRank:  entry.Rank,
			Change:   oldRank - entry.Rank,
		})
	}
	sort.Slice(out, func(ii, jj int) bool {
		return out[ii].Change > out[jj].Change
	})
	return out
}
