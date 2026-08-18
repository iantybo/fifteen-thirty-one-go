package stats

import (
	"errors"
	"math"
	"sort"
)

// HistogramOptions controls construction.
type HistogramOptions struct {
	Min     float64
	Max     float64
	Buckets int
}

// BuildHistogram builds a histogram from raw values with fixed-width buckets.
func BuildHistogram(values []float64, opts HistogramOptions) (Histogram, error) {
	if opts.Buckets <= 0 {
		return Histogram{}, errors.New("buckets must be > 0")
	}
	if len(values) == 0 {
		return Histogram{Buckets: make([]HistogramBucket, opts.Buckets)}, nil
	}
	mn := opts.Min
	mx := opts.Max
	if mn == 0 && mx == 0 {
		mn, mx = minMax(values)
	}
	if mx <= mn {
		mx = mn + 1
	}
	width := (mx - mn) / float64(opts.Buckets)
	buckets := make([]HistogramBucket, opts.Buckets)
	for i := range buckets {
		buckets[i].LowerInclusive = mn + float64(i)*width
		buckets[i].UpperExclusive = mn + float64(i+1)*width
	}
	total := 0
	for _, v := range values {
		if v < mn || v >= mx {
			if v == mx {
				buckets[opts.Buckets-1].Count++
				total++
			}
			continue
		}
		idx := int(math.Floor((v - mn) / width))
		if idx < 0 {
			idx = 0
		}
		if idx >= opts.Buckets {
			idx = opts.Buckets - 1
		}
		buckets[idx].Count++
		total++
	}
	return Histogram{Buckets: buckets, Total: total}, nil
}

func minMax(xs []float64) (float64, float64) {
	mn := xs[0]
	mx := xs[0]
	for _, x := range xs[1:] {
		if x < mn {
			mn = x
		}
		if x > mx {
			mx = x
		}
	}
	return mn, mx
}

// ScoreHistogram builds a histogram of player scores for the supplied games.
func ScoreHistogram(games []Game, buckets int) (Histogram, error) {
	values := make([]float64, len(games))
	for i, g := range games {
		values[i] = float64(g.PlayerScore)
	}
	return BuildHistogram(values, HistogramOptions{Buckets: buckets})
}

// DurationHistogram builds a histogram of game durations in seconds.
func DurationHistogram(games []Game, buckets int) (Histogram, error) {
	values := make([]float64, len(games))
	for i, g := range games {
		values[i] = g.Duration().Seconds()
	}
	return BuildHistogram(values, HistogramOptions{Buckets: buckets})
}

// Mean returns the mean of the slice (0 if empty).
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var s float64
	for _, v := range values {
		s += v
	}
	return s / float64(len(values))
}

// Median returns the median (copies and sorts the slice).
func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := make([]float64, len(values))
	copy(cp, values)
	sort.Float64s(cp)
	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2.0
}

// Percentile computes the value at percentile p (0..100).
func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return values[0]
	}
	if p >= 100 {
		return values[len(values)-1]
	}
	cp := make([]float64, len(values))
	copy(cp, values)
	sort.Float64s(cp)
	rank := (p / 100.0) * float64(len(cp)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return cp[lo]
	}
	frac := rank - float64(lo)
	return cp[lo] + (cp[hi]-cp[lo])*frac
}

// StdDev returns the sample standard deviation.
func StdDev(values []float64) float64 {
	return stdDev(values)
}

// Summary returns simple descriptive statistics for a slice of values.
type Summary struct {
	N      int
	Mean   float64
	Median float64
	Min    float64
	Max    float64
	StdDev float64
	P25    float64
	P75    float64
	P95    float64
	P99    float64
}

// Describe returns a Summary of the provided values.
func Describe(values []float64) Summary {
	if len(values) == 0 {
		return Summary{}
	}
	mn, mx := minMax(values)
	return Summary{
		N:      len(values),
		Mean:   Mean(values),
		Median: Median(values),
		Min:    mn,
		Max:    mx,
		StdDev: StdDev(values),
		P25:    Percentile(values, 25),
		P75:    Percentile(values, 75),
		P95:    Percentile(values, 95),
		P99:    Percentile(values, 99),
	}
}
