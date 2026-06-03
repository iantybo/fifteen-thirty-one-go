package stats

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"
)

func mustTime(t *testing.T, timeStr string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}

func makeGame(id, playerID, opponentID string, playerScore, oppScore int, res GameResult, end time.Time) Game {
	return Game{
		ID: id, Mode: ModeRanked, PlayerID: playerID, OpponentID: opponentID,
		PlayerScore: playerScore, OppScore: oppScore, Result: res,
		StartedAt: end.Add(-5 * time.Minute), EndedAt: end,
		DurationSec: 300, Moves: 30,
		RatingBefore: 1200, RatingAfter: 1210,
	}
}

func TestExpectedSymmetric(t *testing.T) {
	score := Expected(1200, 1200)
	if math.Abs(score-0.5) > 1e-9 {
		t.Fatalf("want 0.5, got %v", score)
	}
}

func TestUpdateBounds(t *testing.T) {
	cfg := DefaultRatingConfig()
	delta := Update(cfg, 1500, 1300, ResultWin)
	if delta <= 0 {
		t.Fatalf("expected positive delta, got %d", delta)
	}
}

func TestSummarizeBasic(t *testing.T) {
	now := mustTime(t, "2026-06-01T12:00:00Z")
	g1 := makeGame("g1", "alice", "bob", 31, 22, ResultWin, now.Add(-48*time.Hour))
	g2 := makeGame("g2", "alice", "bob", 18, 31, ResultLoss, now.Add(-24*time.Hour))
	g3 := makeGame("g3", "alice", "carol", 27, 27, ResultDraw, now.Add(-2*time.Hour))

	agg := NewAggregatorWithClock(func() time.Time { return now })
	agg.AddMany([]Game{g1, g2, g3})
	summary := agg.Summarize("alice")
	if summary.Games != 3 {
		t.Fatalf("games=%d", summary.Games)
	}
	if summary.Wins != 1 || summary.Losses != 1 || summary.Draws != 1 {
		t.Fatalf("wld=%d/%d/%d", summary.Wins, summary.Losses, summary.Draws)
	}
	if math.Abs(summary.WinRate-1.0/3.0) > 1e-9 {
		t.Fatalf("winrate=%v", summary.WinRate)
	}
}

func TestHistogram(t *testing.T) {
	hist, err := BuildHistogram([]float64{1, 2, 3, 4, 5}, HistogramOptions{Buckets: 5, Min: 1, Max: 6})
	if err != nil {
		t.Fatal(err)
	}
	if hist.Total != 5 {
		t.Fatalf("total=%d", hist.Total)
	}
}

func TestStore(t *testing.T) {
	store := NewStore()
	game := makeGame("g1", "a", "b", 10, 5, ResultWin, time.Now())
	if err := store.Insert(game); err != nil {
		t.Fatal(err)
	}
	if store.Count() != 1 {
		t.Fatalf("count=%d", store.Count())
	}
	if _, ok := store.Get("g1"); !ok {
		t.Fatal("missing g1")
	}
	if !store.Delete("g1") {
		t.Fatal("delete failed")
	}
	if store.Count() != 0 {
		t.Fatalf("count after delete=%d", store.Count())
	}
}

func TestEngineRecord(t *testing.T) {
	eng := NewEngine(DefaultRatingConfig())
	game := makeGame("g1", "a", "b", 30, 10, ResultWin, time.Now())
	playerARating, playerBRating := eng.Record(game)
	if playerARating <= 1200 {
		t.Fatalf("expected A rating up, got %d", playerARating)
	}
	if playerBRating >= 1200 {
		t.Fatalf("expected B rating down, got %d", playerBRating)
	}
}

func TestCSVRoundTrip(t *testing.T) {
	games := []Game{
		makeGame("g1", "a", "b", 10, 5, ResultWin, mustTime(t, "2026-01-01T00:00:00Z")),
		makeGame("g2", "a", "b", 5, 10, ResultLoss, mustTime(t, "2026-01-02T00:00:00Z")),
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, games); err != nil {
		t.Fatal(err)
	}
	out, err := ReadCSV(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d", len(out))
	}
	if out[0].ID != "g1" || out[1].Result != ResultLoss {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestSeriesBuilder(t *testing.T) {
	builder := NewSeriesBuilder(GranularityDay)
	t0 := mustTime(t, "2026-01-01T10:00:00Z")
	builder.Add(t0, 5)
	builder.Add(t0.Add(2*time.Hour), 3)
	builder.Add(t0.Add(48*time.Hour), 1)
	series := builder.Build(false)
	if len(series.Points) != 2 {
		t.Fatalf("got %d points", len(series.Points))
	}
	if series.Points[0].Value != 8 {
		t.Fatalf("first bucket sum=%v", series.Points[0].Value)
	}
}

func TestLeaderboard(t *testing.T) {
	now := mustTime(t, "2026-06-01T00:00:00Z")
	eng := NewEngine(DefaultRatingConfig())
	eng.Set("a", 1500)
	eng.Set("b", 1700)
	eng.Set("c", 1400)
	sums := map[string]PlayerSummary{
		"a": {PlayerID: "a", Games: 10, Wins: 7, WinRate: 0.7},
		"b": {PlayerID: "b", Games: 12, Wins: 6, WinRate: 0.5},
		"c": {PlayerID: "c", Games: 8, Wins: 4, WinRate: 0.5},
	}
	lb, err := BuildLeaderboard(sums, eng, DefaultLeaderboardOptions(), now)
	if err != nil {
		t.Fatal(err)
	}
	if lb.Entries[0].PlayerID != "b" {
		t.Fatalf("expected b first, got %s", lb.Entries[0].PlayerID)
	}
}

func TestFormatSummary(t *testing.T) {
	summary := PlayerSummary{
		PlayerID: "alice", Games: 10, Wins: 7, Losses: 2, Draws: 1, WinRate: 0.7,
		AvgScore: 25, AvgOppScore: 20, AvgScoreDelta: 5,
		CurrentRating: 1500, PeakRating: 1550, LowestRating: 1200,
	}
	out := FormatSummary(summary)
	if !strings.Contains(out, "alice") {
		t.Fatalf("missing alice: %s", out)
	}
}

func TestHeadToHead(t *testing.T) {
	now := time.Now()
	games := []Game{
		makeGame("g1", "a", "b", 30, 20, ResultWin, now),
		makeGame("g2", "a", "b", 10, 20, ResultLoss, now.Add(time.Hour)),
		makeGame("g3", "b", "a", 30, 10, ResultWin, now.Add(2*time.Hour)),
	}
	h2h := ComputeHeadToHead(games, "a", "b")
	if h2h.Games != 3 {
		t.Fatalf("games=%d", h2h.Games)
	}
	if h2h.AWins != 1 || h2h.BWins != 2 {
		t.Fatalf("AW=%d BW=%d", h2h.AWins, h2h.BWins)
	}
}

func TestSeriesMovingAverage(t *testing.T) {
	series := Series{
		Granularity: GranularityDay,
		Points: []SeriesPoint{
			{Value: 1}, {Value: 2}, {Value: 3}, {Value: 4}, {Value: 5},
		},
	}
	ma := series.MovingAverage(3)
	if ma.Points[4].Value != 4 {
		t.Fatalf("expected 4, got %v", ma.Points[4].Value)
	}
}
