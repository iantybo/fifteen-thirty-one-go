package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// FormatPercent renders a 0..1 value as a percent string with one decimal.
func FormatPercent(v float64) string {
	return fmt.Sprintf("%.1f%%", v*100.0)
}

// FormatDuration renders a duration as Hh Mm Ss compactly.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	parts := make([]string, 0, 3)
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if s > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}
	return strings.Join(parts, " ")
}

// FormatStreak renders a streak in a user-friendly way.
func FormatStreak(s Streak) string {
	if s.Length == 0 {
		return "no streak"
	}
	return fmt.Sprintf("%d %s streak since %s", s.Length, s.Result.String(), s.Since.Format("2006-01-02"))
}

// FormatSummary renders a multiline player summary.
func FormatSummary(s PlayerSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Player: %s\n", s.PlayerID)
	fmt.Fprintf(&b, "  Games:        %d  (W:%d L:%d D:%d)\n", s.Games, s.Wins, s.Losses, s.Draws)
	fmt.Fprintf(&b, "  Win rate:     %s\n", FormatPercent(s.WinRate))
	fmt.Fprintf(&b, "  Avg score:    %.2f vs %.2f (delta %.2f)\n", s.AvgScore, s.AvgOppScore, s.AvgScoreDelta)
	fmt.Fprintf(&b, "  Rating:       %d (peak %d, low %d, 30d Δ %+d)\n", s.CurrentRating, s.PeakRating, s.LowestRating, s.RatingDelta30Day)
	fmt.Fprintf(&b, "  Streak:       %s\n", FormatStreak(s.CurrentStreak))
	fmt.Fprintf(&b, "  Longest win:  %d   Longest loss: %d\n", s.LongestWinStreak, s.LongestLossStreak)
	fmt.Fprintf(&b, "  Play time:    %s (avg %s)\n", FormatDuration(s.TotalPlayTime), FormatDuration(s.AvgGameDuration))
	if len(s.ModeBreakdown) > 0 {
		fmt.Fprintln(&b, "  By mode:")
		modes := make([]GameMode, 0, len(s.ModeBreakdown))
		for m := range s.ModeBreakdown {
			modes = append(modes, m)
		}
		sort.Slice(modes, func(i, j int) bool { return modes[i] < modes[j] })
		for _, m := range modes {
			ms := s.ModeBreakdown[m]
			fmt.Fprintf(&b, "    %-12s %3d games  WR %s  avg %.2f\n", ms.Mode.String(), ms.Games, FormatPercent(ms.WinRate), ms.AvgScore)
		}
	}
	return b.String()
}

// FormatLeaderboard renders a leaderboard as a fixed-width table.
func FormatLeaderboard(l Leaderboard, top int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Leaderboard (%s) — generated %s\n", l.Mode.String(), l.UpdatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "%-5s %-20s %-7s %-7s %-9s %-7s\n", "Rank", "Player", "Rating", "Games", "WinRate", "Trend")
	entries := l.Top(top)
	for _, e := range entries {
		fmt.Fprintf(&b, "%-5d %-20s %-7d %-7d %-9s %+7d\n", e.Rank, truncate(e.PlayerID, 20), e.Rating, e.Games, FormatPercent(e.WinRate), e.Trend)
	}
	return b.String()
}

// FormatHeadToHead renders a head-to-head summary.
func FormatHeadToHead(h HeadToHead) string {
	if h.Games == 0 {
		return fmt.Sprintf("%s vs %s — no games played", h.PlayerA, h.PlayerB)
	}
	return fmt.Sprintf("%s vs %s — %d games (W:%d L:%d D:%d) avg margin %.2f, last met %s",
		h.PlayerA, h.PlayerB, h.Games, h.AWins, h.BWins, h.Draws, h.AvgMargin, h.LastMet.Format("2006-01-02"))
}

// FormatHistogram renders a horizontal text histogram.
func FormatHistogram(h Histogram, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 40
	}
	if len(h.Buckets) == 0 {
		return "(empty histogram)"
	}
	var b strings.Builder
	max := 0
	for _, bk := range h.Buckets {
		if bk.Count > max {
			max = bk.Count
		}
	}
	if max == 0 {
		max = 1
	}
	for _, bk := range h.Buckets {
		width := bk.Count * maxWidth / max
		fmt.Fprintf(&b, "[%6.1f - %6.1f) | %s %d\n",
			bk.LowerInclusive, bk.UpperExclusive,
			strings.Repeat("#", width), bk.Count)
	}
	return b.String()
}

// FormatSeries renders the points in a series.
func FormatSeries(s Series) string {
	var b strings.Builder
	for _, p := range s.Points {
		fmt.Fprintf(&b, "%s  %10.3f  (n=%d)\n", p.At.Format(time.RFC3339), p.Value, p.Count)
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return s[:n-1] + "…"
}
