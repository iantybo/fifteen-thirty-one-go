package stats

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return v
}

func makeGame(id, p, o string, ps, os int, res GameResult, end time.Time) Game {
	return Game{
		ID: id, Mode: ModeRanked, PlayerID: p, OpponentID: o,
		PlayerScore: ps, OppScore: os, Result: res,
		StartedAt: end.Add(-5 * time.Minute), EndedAt: end,
		DurationSec: 300, Moves: 30,
		RatingBefore: 1200, RatingAfter: 1210,
	}
}

func TestExpectedSymmetric(t *testing.T) {
	a := Expected(1200, 1200)
	if math.Abs(a-0.5) > 1e-9 {
		t.Fatalf("want 0.5, got %v", a)
	}
}

func TestUpdateBounds(t *testing.T) {
	cfg := DefaultRatingConfig()
	d := Update(cfg, 1500, 1300, ResultWin)
	if d <= 0 {
		t.Fatalf("expected positive delta, got %d", d)
	}
}

func TestSummarizeBasic(t *testing.T) {
	now := mustTime(t, "2026-06-01T12:00:00Z")
	g1 := makeGame("g1", "alice", "bob", 31, 22, ResultWin, now.Add(-48*time.Hour))
	g2 := makeGame("g2", "alice", "bob", 18, 31, ResultLoss, now.Add(-24*time.Hour))
	g3 := makeGame("g3", "alice", "carol", 27, 27, ResultDraw, now.Add(-2*time.Hour))

	a := NewAggregatorWithClock(func() time.Time { return now })
	a.AddMany([]Game{g1, g2, g3})
	s := a.Summarize("alice")
	if s.Games != 3 {
		t.Fatalf("games=%d", s.Games)
	}
	if s.Wins != 1 || s.Losses != 1 || s.Draws != 1 {
		t.Fatalf("wld=%d/%d/%d", s.Wins, s.Losses, s.Draws)
	}
	if math.Abs(s.WinRate-1.0/3.0) > 1e-9 {
		t.Fatalf("winrate=%v", s.WinRate)
	}
}

func TestHistogram(t *testing.T) {
	h, err := BuildHistogram([]float64{1, 2, 3, 4, 5}, HistogramOptions{Buckets: 5, Min: 1, Max: 6})
	if err != nil {
		t.Fatal(err)
	}
	if h.Total != 5 {
		t.Fatalf("total=%d", h.Total)
	}
}

func TestStore(t *testing.T) {
	s := NewStore()
	g := makeGame("g1", "a", "b", 10, 5, ResultWin, time.Now())
	if err := s.Insert(g); err != nil {
		t.Fatal(err)
	}
	if s.Count() != 1 {
		t.Fatalf("count=%d", s.Count())
	}
	if _, ok := s.Get("g1"); !ok {
		t.Fatal("missing g1")
	}
	if !s.Delete("g1") {
		t.Fatal("delete failed")
	}
	if s.Count() != 0 {
		t.Fatalf("count after delete=%d", s.Count())
	}
}

func TestEngineRecord(t *testing.T) {
	e := NewEngine(DefaultRatingConfig())
	g := makeGame("g1", "a", "b", 30, 10, ResultWin, time.Now())
	pA, pB := e.Record(g)
	if pA <= 1200 {
		t.Fatalf("expected A rating up, got %d", pA)
	}
	if pB >= 1200 {
		t.Fatalf("expected B rating down, got %d", pB)
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
	b := NewSeriesBuilder(GranularityDay)
	t0 := mustTime(t, "2026-01-01T10:00:00Z")
	b.Add(t0, 5)
	b.Add(t0.Add(2*time.Hour), 3)
	b.Add(t0.Add(48*time.Hour), 1)
	s := b.Build(false)
	if len(s.Points) != 2 {
		t.Fatalf("got %d points", len(s.Points))
	}
	if s.Points[0].Value != 8 {
		t.Fatalf("first bucket sum=%v", s.Points[0].Value)
	}
}

func TestLeaderboard(t *testing.T) {
	now := mustTime(t, "2026-06-01T00:00:00Z")
	e := NewEngine(DefaultRatingConfig())
	e.Set("a", 1500)
	e.Set("b", 1700)
	e.Set("c", 1400)
	sums := map[string]PlayerSummary{
		"a": {PlayerID: "a", Games: 10, Wins: 7, WinRate: 0.7},
		"b": {PlayerID: "b", Games: 12, Wins: 6, WinRate: 0.5},
		"c": {PlayerID: "c", Games: 8, Wins: 4, WinRate: 0.5},
	}
	lb, err := BuildLeaderboard(sums, e, DefaultLeaderboardOptions(), now)
	if err != nil {
		t.Fatal(err)
	}
	if lb.Entries[0].PlayerID != "b" {
		t.Fatalf("expected b first, got %s", lb.Entries[0].PlayerID)
	}
}

func TestFormatSummary(t *testing.T) {
	s := PlayerSummary{
		PlayerID: "alice", Games: 10, Wins: 7, Losses: 2, Draws: 1, WinRate: 0.7,
		AvgScore: 25, AvgOppScore: 20, AvgScoreDelta: 5,
		CurrentRating: 1500, PeakRating: 1550, LowestRating: 1200,
	}
	out := FormatSummary(s)
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
	h := ComputeHeadToHead(games, "a", "b")
	if h.Games != 3 {
		t.Fatalf("games=%d", h.Games)
	}
	if h.AWins != 1 || h.BWins != 2 {
		t.Fatalf("AW=%d BW=%d", h.AWins, h.BWins)
	}
}

func TestSeriesMovingAverage(t *testing.T) {
	s := Series{
		Granularity: GranularityDay,
		Points: []SeriesPoint{
			{Value: 1}, {Value: 2}, {Value: 3}, {Value: 4}, {Value: 5},
		},
	}
	m := s.MovingAverage(3)
	if m.Points[4].Value != 4 {
		t.Fatalf("expected 4, got %v", m.Points[4].Value)
	}
}
