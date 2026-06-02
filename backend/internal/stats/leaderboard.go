package stats

import (
	"errors"
	"sort"
	"time"
)

// LeaderboardOptions controls leaderboard construction.
type LeaderboardOptions struct {
	Mode       GameMode
	MinGames   int
	Limit      int
	ActiveOnly bool
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
	for player, s := range summaries {
		if opts.MinGames > 0 && s.Games < opts.MinGames {
			continue
		}
		if opts.ActiveOnly && !opts.ActiveSince.IsZero() && s.LastPlayed.Before(opts.ActiveSince) {
			continue
		}
		ms := s.ModeBreakdown[opts.Mode]
		winRate := s.WinRate
		games := s.Games
		if opts.Mode != ModeUnknown && ms.Games > 0 {
			winRate = ms.WinRate
			games = ms.Games
		}
		rating := engine.Get(player)
		entries = append(entries, LeaderboardEntry{
			PlayerID: player,
			Rating:   rating,
			Games:    games,
			WinRate:  winRate,
			LastSeen: s.LastPlayed,
			Trend:    s.RatingDelta30Day,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Rating != entries[j].Rating {
			return entries[i].Rating > entries[j].Rating
		}
		if entries[i].Games != entries[j].Games {
			return entries[i].Games > entries[j].Games
		}
		return entries[i].PlayerID < entries[j].PlayerID
	})

	for i := range entries {
		entries[i].Rank = i + 1
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
func (l Leaderboard) FindEntry(playerID string) (LeaderboardEntry, bool) {
	for _, e := range l.Entries {
		if e.PlayerID == playerID {
			return e, true
		}
	}
	return LeaderboardEntry{}, false
}

// Top returns the first n entries.
func (l Leaderboard) Top(n int) []LeaderboardEntry {
	if n <= 0 || n >= len(l.Entries) {
		return l.Entries
	}
	return l.Entries[:n]
}

// Around returns entries surrounding the rank of the specified player.
func (l Leaderboard) Around(playerID string, span int) []LeaderboardEntry {
	if span < 0 {
		span = 0
	}
	for i, e := range l.Entries {
		if e.PlayerID == playerID {
			lo := i - span
			if lo < 0 {
				lo = 0
			}
			hi := i + span + 1
			if hi > len(l.Entries) {
				hi = len(l.Entries)
			}
			return l.Entries[lo:hi]
		}
	}
	return nil
}

// MultiLeaderboards builds one leaderboard per mode.
func MultiLeaderboards(summaries map[string]PlayerSummary, engine *Engine, modes []GameMode, base LeaderboardOptions, now time.Time) (map[GameMode]Leaderboard, error) {
	out := make(map[GameMode]Leaderboard, len(modes))
	for _, m := range modes {
		opts := base
		opts.Mode = m
		lb, err := BuildLeaderboard(summaries, engine, opts, now)
		if err != nil {
			return nil, err
		}
		out[m] = lb
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
	for _, e := range prev.Entries {
		prevRanks[e.PlayerID] = e.Rank
	}
	var out []Movement
	for _, e := range curr.Entries {
		old, ok := prevRanks[e.PlayerID]
		if !ok {
			out = append(out, Movement{PlayerID: e.PlayerID, OldRank: 0, NewRank: e.Rank, Change: 0})
			continue
		}
		out = append(out, Movement{
			PlayerID: e.PlayerID,
			OldRank:  old,
			NewRank:  e.Rank,
			Change:   old - e.Rank,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Change > out[j].Change
	})
	return out
}
