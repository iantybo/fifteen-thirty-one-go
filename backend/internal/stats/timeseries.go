package stats

import (
	"errors"
	"sort"
	"time"
)

// Granularity controls bucketing for time series.
type Granularity int

const (
	GranularityHour Granularity = iota
	GranularityDay
	GranularityWeek
	GranularityMonth
)

// Truncate snaps a time down to the granularity boundary.
func (g Granularity) Truncate(t time.Time) time.Time {
	switch g {
	case GranularityHour:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case GranularityDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case GranularityWeek:
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		offset := int(d.Weekday())
		return d.AddDate(0, 0, -offset)
	case GranularityMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	}
	return t
}

// Step returns the duration of one bucket.
func (g Granularity) Step() time.Duration {
	switch g {
	case GranularityHour:
		return time.Hour
	case GranularityDay:
		return 24 * time.Hour
	case GranularityWeek:
		return 7 * 24 * time.Hour
	case GranularityMonth:
		return 30 * 24 * time.Hour
	}
	return time.Hour
}

// SeriesPoint is one point in a time-bucketed series.
type SeriesPoint struct {
	At     time.Time
	Value  float64
	Count  int
}

// Series is a list of time-bucketed points.
type Series struct {
	Granularity Granularity
	Points      []SeriesPoint
}

// SeriesBuilder accumulates per-bucket counts and sums.
type SeriesBuilder struct {
	gran     Granularity
	buckets  map[time.Time]*bucketData
}

type bucketData struct {
	sum   float64
	count int
}

// NewSeriesBuilder builds a new SeriesBuilder.
func NewSeriesBuilder(g Granularity) *SeriesBuilder {
	return &SeriesBuilder{
		gran:    g,
		buckets: make(map[time.Time]*bucketData),
	}
}

// Add adds a single value at time t.
func (b *SeriesBuilder) Add(t time.Time, value float64) {
	key := b.gran.Truncate(t)
	bd, ok := b.buckets[key]
	if !ok {
		bd = &bucketData{}
		b.buckets[key] = bd
	}
	bd.sum += value
	bd.count++
}

// Build returns a sorted series. If avg is true, values are averaged per bucket.
func (b *SeriesBuilder) Build(avg bool) Series {
	pts := make([]SeriesPoint, 0, len(b.buckets))
	for k, v := range b.buckets {
		val := v.sum
		if avg && v.count > 0 {
			val = v.sum / float64(v.count)
		}
		pts = append(pts, SeriesPoint{At: k, Value: val, Count: v.count})
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].At.Before(pts[j].At) })
	return Series{Granularity: b.gran, Points: pts}
}

// FillGaps inserts zero-value points for any missing buckets in [from,to].
func (s Series) FillGaps(from, to time.Time) Series {
	if len(s.Points) == 0 {
		return s
	}
	step := s.Granularity.Step()
	existing := make(map[time.Time]SeriesPoint, len(s.Points))
	for _, p := range s.Points {
		existing[p.At] = p
	}
	var out []SeriesPoint
	for t := s.Granularity.Truncate(from); !t.After(to); t = t.Add(step) {
		if p, ok := existing[t]; ok {
			out = append(out, p)
		} else {
			out = append(out, SeriesPoint{At: t})
		}
	}
	return Series{Granularity: s.Granularity, Points: out}
}

// MovingAverage returns a new series smoothed via a sliding window.
func (s Series) MovingAverage(window int) Series {
	if window <= 1 {
		return s
	}
	out := make([]SeriesPoint, len(s.Points))
	for i := range s.Points {
		lo := i - window + 1
		if lo < 0 {
			lo = 0
		}
		var sum float64
		var n int
		for j := lo; j <= i; j++ {
			sum += s.Points[j].Value
			n++
		}
		out[i] = SeriesPoint{At: s.Points[i].At, Value: sum / float64(n), Count: s.Points[i].Count}
	}
	return Series{Granularity: s.Granularity, Points: out}
}

// CumulativeSum returns a series whose values are the running totals.
func (s Series) CumulativeSum() Series {
	out := make([]SeriesPoint, len(s.Points))
	var run float64
	for i, p := range s.Points {
		run += p.Value
		out[i] = SeriesPoint{At: p.At, Value: run, Count: p.Count}
	}
	return Series{Granularity: s.Granularity, Points: out}
}

// Max returns the maximum value in the series.
func (s Series) Max() (SeriesPoint, bool) {
	if len(s.Points) == 0 {
		return SeriesPoint{}, false
	}
	m := s.Points[0]
	for _, p := range s.Points[1:] {
		if p.Value > m.Value {
			m = p
		}
	}
	return m, true
}

// Min returns the minimum value in the series.
func (s Series) Min() (SeriesPoint, bool) {
	if len(s.Points) == 0 {
		return SeriesPoint{}, false
	}
	m := s.Points[0]
	for _, p := range s.Points[1:] {
		if p.Value < m.Value {
			m = p
		}
	}
	return m, true
}

// GamesPerPeriod builds a series of game counts.
func GamesPerPeriod(games []Game, g Granularity) Series {
	b := NewSeriesBuilder(g)
	for _, gm := range games {
		b.Add(gm.EndedAt, 1)
	}
	return b.Build(false)
}

// WinRatePerPeriod builds a per-period win rate series for a given player.
func WinRatePerPeriod(games []Game, playerID string, g Granularity) Series {
	winBuilder := NewSeriesBuilder(g)
	gameBuilder := NewSeriesBuilder(g)
	for _, gm := range games {
		if gm.PlayerID != playerID {
			continue
		}
		gameBuilder.Add(gm.EndedAt, 1)
		if gm.Result == ResultWin {
			winBuilder.Add(gm.EndedAt, 1)
		}
	}
	wins := winBuilder.Build(false)
	gs := gameBuilder.Build(false)
	winsByT := make(map[time.Time]SeriesPoint, len(wins.Points))
	for _, p := range wins.Points {
		winsByT[p.At] = p
	}
	out := make([]SeriesPoint, 0, len(gs.Points))
	for _, p := range gs.Points {
		w := winsByT[p.At]
		var rate float64
		if p.Count > 0 {
			rate = float64(w.Count) / float64(p.Count)
		}
		out = append(out, SeriesPoint{At: p.At, Value: rate, Count: p.Count})
	}
	return Series{Granularity: g, Points: out}
}

// MergeSeries sums multiple series elementwise (aligned on truncated timestamps).
func MergeSeries(series ...Series) (Series, error) {
	if len(series) == 0 {
		return Series{}, errors.New("no series")
	}
	g := series[0].Granularity
	totals := make(map[time.Time]*bucketData)
	for _, s := range series {
		if s.Granularity != g {
			return Series{}, errors.New("granularity mismatch")
		}
		for _, p := range s.Points {
			bd, ok := totals[p.At]
			if !ok {
				bd = &bucketData{}
				totals[p.At] = bd
			}
			bd.sum += p.Value
			bd.count += p.Count
		}
	}
	pts := make([]SeriesPoint, 0, len(totals))
	for k, v := range totals {
		pts = append(pts, SeriesPoint{At: k, Value: v.sum, Count: v.count})
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].At.Before(pts[j].At) })
	return Series{Granularity: g, Points: pts}, nil
}
