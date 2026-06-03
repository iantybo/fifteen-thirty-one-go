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
func (gran Granularity) Truncate(ts time.Time) time.Time {
	switch gran {
	case GranularityHour:
		return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), 0, 0, 0, ts.Location())
	case GranularityDay:
		return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location())
	case GranularityWeek:
		weekStart := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location())
		offset := int(weekStart.Weekday())
		return weekStart.AddDate(0, 0, -offset)
	case GranularityMonth:
		return time.Date(ts.Year(), ts.Month(), 1, 0, 0, 0, 0, ts.Location())
	}
	return ts
}

// Step returns the duration of one bucket.
func (gran Granularity) Step() time.Duration {
	switch gran {
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
	At    time.Time
	Value float64
	Count int
}

// Series is a list of time-bucketed points.
type Series struct {
	Granularity Granularity
	Points      []SeriesPoint
}

// SeriesBuilder accumulates per-bucket counts and sums.
type SeriesBuilder struct {
	gran    Granularity
	buckets map[time.Time]*bucketData
}

type bucketData struct {
	sum   float64
	count int
}

// NewSeriesBuilder builds a new SeriesBuilder.
func NewSeriesBuilder(gran Granularity) *SeriesBuilder {
	return &SeriesBuilder{
		gran:    gran,
		buckets: make(map[time.Time]*bucketData),
	}
}

// Add adds a single value at time ts.
func (builder *SeriesBuilder) Add(ts time.Time, value float64) {
	key := builder.gran.Truncate(ts)
	bd, ok := builder.buckets[key]
	if !ok {
		bd = &bucketData{}
		builder.buckets[key] = bd
	}
	bd.sum += value
	bd.count++
}

// Build returns a sorted series. If avg is true, values are averaged per bucket.
func (builder *SeriesBuilder) Build(avg bool) Series {
	pts := make([]SeriesPoint, 0, len(builder.buckets))
	for bucketKey, bucketVal := range builder.buckets {
		val := bucketVal.sum
		if avg && bucketVal.count > 0 {
			val = bucketVal.sum / float64(bucketVal.count)
		}
		pts = append(pts, SeriesPoint{At: bucketKey, Value: val, Count: bucketVal.count})
	}
	sort.Slice(pts, func(ii, jj int) bool { return pts[ii].At.Before(pts[jj].At) })
	return Series{Granularity: builder.gran, Points: pts}
}

// FillGaps inserts zero-value points for any missing buckets in [from,to].
func (series Series) FillGaps(from, to time.Time) Series {
	if len(series.Points) == 0 {
		return series
	}
	step := series.Granularity.Step()
	existing := make(map[time.Time]SeriesPoint, len(series.Points))
	for _, pt := range series.Points {
		existing[pt.At] = pt
	}
	var out []SeriesPoint
	for ts := series.Granularity.Truncate(from); !ts.After(to); ts = ts.Add(step) {
		if pt, ok := existing[ts]; ok {
			out = append(out, pt)
		} else {
			out = append(out, SeriesPoint{At: ts})
		}
	}
	return Series{Granularity: series.Granularity, Points: out}
}

// MovingAverage returns a new series smoothed via a sliding window.
func (series Series) MovingAverage(window int) Series {
	if window <= 1 {
		return series
	}
	out := make([]SeriesPoint, len(series.Points))
	for idx := range series.Points {
		lo := idx - window + 1
		if lo < 0 {
			lo = 0
		}
		var sum float64
		var count int
		for jj := lo; jj <= idx; jj++ {
			sum += series.Points[jj].Value
			count++
		}
		out[idx] = SeriesPoint{At: series.Points[idx].At, Value: sum / float64(count), Count: series.Points[idx].Count}
	}
	return Series{Granularity: series.Granularity, Points: out}
}

// CumulativeSum returns a series whose values are the running totals.
func (series Series) CumulativeSum() Series {
	out := make([]SeriesPoint, len(series.Points))
	var run float64
	for idx, pt := range series.Points {
		run += pt.Value
		out[idx] = SeriesPoint{At: pt.At, Value: run, Count: pt.Count}
	}
	return Series{Granularity: series.Granularity, Points: out}
}

// Max returns the maximum value in the series.
func (series Series) Max() (SeriesPoint, bool) {
	if len(series.Points) == 0 {
		return SeriesPoint{}, false
	}
	maxPt := series.Points[0]
	for _, pt := range series.Points[1:] {
		if pt.Value > maxPt.Value {
			maxPt = pt
		}
	}
	return maxPt, true
}

// Min returns the minimum value in the series.
func (series Series) Min() (SeriesPoint, bool) {
	if len(series.Points) == 0 {
		return SeriesPoint{}, false
	}
	minPt := series.Points[0]
	for _, pt := range series.Points[1:] {
		if pt.Value < minPt.Value {
			minPt = pt
		}
	}
	return minPt, true
}

// GamesPerPeriod builds a series of game counts.
func GamesPerPeriod(games []Game, gran Granularity) Series {
	builder := NewSeriesBuilder(gran)
	for _, game := range games {
		builder.Add(game.EndedAt, 1)
	}
	return builder.Build(false)
}

// WinRatePerPeriod builds a per-period win rate series for a given player.
func WinRatePerPeriod(games []Game, playerID string, gran Granularity) Series {
	winBuilder := NewSeriesBuilder(gran)
	gameBuilder := NewSeriesBuilder(gran)
	for _, game := range games {
		if game.PlayerID != playerID {
			continue
		}
		gameBuilder.Add(game.EndedAt, 1)
		if game.Result == ResultWin {
			winBuilder.Add(game.EndedAt, 1)
		}
	}
	wins := winBuilder.Build(false)
	gameSeries := gameBuilder.Build(false)
	winsByTime := make(map[time.Time]SeriesPoint, len(wins.Points))
	for _, pt := range wins.Points {
		winsByTime[pt.At] = pt
	}
	out := make([]SeriesPoint, 0, len(gameSeries.Points))
	for _, pt := range gameSeries.Points {
		winPt := winsByTime[pt.At]
		var rate float64
		if pt.Count > 0 {
			rate = float64(winPt.Count) / float64(pt.Count)
		}
		out = append(out, SeriesPoint{At: pt.At, Value: rate, Count: pt.Count})
	}
	return Series{Granularity: gran, Points: out}
}

// MergeSeries sums multiple series elementwise (aligned on truncated timestamps).
func MergeSeries(allSeries ...Series) (Series, error) {
	if len(allSeries) == 0 {
		return Series{}, errors.New("no series")
	}
	gran := allSeries[0].Granularity
	totals := make(map[time.Time]*bucketData)
	for _, ser := range allSeries {
		if ser.Granularity != gran {
			return Series{}, errors.New("granularity mismatch")
		}
		for _, pt := range ser.Points {
			bd, ok := totals[pt.At]
			if !ok {
				bd = &bucketData{}
				totals[pt.At] = bd
			}
			bd.sum += pt.Value
			bd.count += pt.Count
		}
	}
	pts := make([]SeriesPoint, 0, len(totals))
	for bucketKey, bucketVal := range totals {
		pts = append(pts, SeriesPoint{At: bucketKey, Value: bucketVal.sum, Count: bucketVal.count})
	}
	sort.Slice(pts, func(ii, jj int) bool { return pts[ii].At.Before(pts[jj].At) })
	return Series{Granularity: gran, Points: pts}, nil
}
