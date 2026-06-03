package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// FormatPercent renders a 0..1 value as a percent string with one decimal.
func FormatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value*100.0)
}

// FormatDuration renders a duration as Hh Mm Ss compactly.
func FormatDuration(dur time.Duration) string {
	if dur <= 0 {
		return "0s"
	}
	hours := int(dur.Hours())
	mins := int(dur.Minutes()) % 60
	secs := int(dur.Seconds()) % 60
	parts := make([]string, 0, 3)
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	if secs > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", secs))
	}
	return strings.Join(parts, " ")
}

// FormatStreak renders a streak in a user-friendly way.
func FormatStreak(streak Streak) string {
	if streak.Length == 0 {
		return "no streak"
	}
	return fmt.Sprintf("%d %s streak since %s", streak.Length, streak.Result.String(), streak.Since.Format("2006-01-02"))
}

// FormatSummary renders a multiline player summary.
func FormatSummary(summary PlayerSummary) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Player: %s\n", summary.PlayerID)
	fmt.Fprintf(&sb, "  Games:        %d  (W:%d L:%d D:%d)\n", summary.Games, summary.Wins, summary.Losses, summary.Draws)
	fmt.Fprintf(&sb, "  Win rate:     %s\n", FormatPercent(summary.WinRate))
	fmt.Fprintf(&sb, "  Avg score:    %.2f vs %.2f (delta %.2f)\n", summary.AvgScore, summary.AvgOppScore, summary.AvgScoreDelta)
	fmt.Fprintf(&sb, "  Rating:       %d (peak %d, low %d, 30d Δ %+d)\n", summary.CurrentRating, summary.PeakRating, summary.LowestRating, summary.RatingDelta30Day)
	fmt.Fprintf(&sb, "  Streak:       %s\n", FormatStreak(summary.CurrentStreak))
	fmt.Fprintf(&sb, "  Longest win:  %d   Longest loss: %d\n", summary.LongestWinStreak, summary.LongestLossStreak)
	fmt.Fprintf(&sb, "  Play time:    %s (avg %s)\n", FormatDuration(summary.TotalPlayTime), FormatDuration(summary.AvgGameDuration))
	if len(summary.ModeBreakdown) > 0 {
		fmt.Fprintln(&sb, "  By mode:")
		modes := make([]GameMode, 0, len(summary.ModeBreakdown))
		for mode := range summary.ModeBreakdown {
			modes = append(modes, mode)
		}
		sort.Slice(modes, func(ii, jj int) bool { return modes[ii] < modes[jj] })
		for _, mode := range modes {
			modeStats := summary.ModeBreakdown[mode]
			fmt.Fprintf(&sb, "    %-12s %3d games  WR %s  avg %.2f\n", modeStats.Mode.String(), modeStats.Games, FormatPercent(modeStats.WinRate), modeStats.AvgScore)
		}
	}
	return sb.String()
}

// FormatLeaderboard renders a leaderboard as a fixed-width table.
func FormatLeaderboard(lb Leaderboard, top int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Leaderboard (%s) — generated %s\n", lb.Mode.String(), lb.UpdatedAt.Format(time.RFC3339))
	fmt.Fprintf(&sb, "%-5s %-20s %-7s %-7s %-9s %-7s\n", "Rank", "Player", "Rating", "Games", "WinRate", "Trend")
	entries := lb.Top(top)
	for _, entry := range entries {
		fmt.Fprintf(&sb, "%-5d %-20s %-7d %-7d %-9s %+7d\n", entry.Rank, truncate(entry.PlayerID, 20), entry.Rating, entry.Games, FormatPercent(entry.WinRate), entry.Trend)
	}
	return sb.String()
}

// FormatHeadToHead renders a head-to-head summary.
func FormatHeadToHead(h2h HeadToHead) string {
	if h2h.Games == 0 {
		return fmt.Sprintf("%s vs %s — no games played", h2h.PlayerA, h2h.PlayerB)
	}
	return fmt.Sprintf("%s vs %s — %d games (W:%d L:%d D:%d) avg margin %.2f, last met %s",
		h2h.PlayerA, h2h.PlayerB, h2h.Games, h2h.AWins, h2h.BWins, h2h.Draws, h2h.AvgMargin, h2h.LastMet.Format("2006-01-02"))
}

// FormatHistogram renders a horizontal text histogram.
func FormatHistogram(hist Histogram, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 40
	}
	if len(hist.Buckets) == 0 {
		return "(empty histogram)"
	}
	var sb strings.Builder
	maxCount := 0
	for _, bk := range hist.Buckets {
		if bk.Count > maxCount {
			maxCount = bk.Count
		}
	}
	if maxCount == 0 {
		maxCount = 1
	}
	for _, bk := range hist.Buckets {
		width := bk.Count * maxWidth / maxCount
		fmt.Fprintf(&sb, "[%6.1f - %6.1f) | %s %d\n",
			bk.LowerInclusive, bk.UpperExclusive,
			strings.Repeat("#", width), bk.Count)
	}
	return sb.String()
}

// FormatSeries renders the points in a series.
func FormatSeries(series Series) string {
	var sb strings.Builder
	for _, pt := range series.Points {
		fmt.Fprintf(&sb, "%s  %10.3f  (n=%d)\n", pt.At.Format(time.RFC3339), pt.Value, pt.Count)
	}
	return sb.String()
}

func truncate(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	if maxLen < 1 {
		return ""
	}
	return str[:maxLen-1] + "…"
}
