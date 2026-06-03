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
	minVal := opts.Min
	maxVal := opts.Max
	if minVal == 0 && maxVal == 0 {
		minVal, maxVal = minMax(values)
	}
	if maxVal <= minVal {
		maxVal = minVal + 1
	}
	width := (maxVal - minVal) / float64(opts.Buckets)
	buckets := make([]HistogramBucket, opts.Buckets)
	for idx := range buckets {
		buckets[idx].LowerInclusive = minVal + float64(idx)*width
		buckets[idx].UpperExclusive = minVal + float64(idx+1)*width
	}
	total := 0
	for _, val := range values {
		if val < minVal || val >= maxVal {
			if val == maxVal {
				buckets[opts.Buckets-1].Count++
				total++
			}
			continue
		}
		bucketIdx := int(math.Floor((val - minVal) / width))
		if bucketIdx < 0 {
			bucketIdx = 0
		}
		if bucketIdx >= opts.Buckets {
			bucketIdx = opts.Buckets - 1
		}
		buckets[bucketIdx].Count++
		total++
	}
	return Histogram{Buckets: buckets, Total: total}, nil
}

func minMax(xs []float64) (float64, float64) {
	minVal := xs[0]
	maxVal := xs[0]
	for _, val := range xs[1:] {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}
	return minVal, maxVal
}

// ScoreHistogram builds a histogram of player scores for the supplied games.
func ScoreHistogram(games []Game, buckets int) (Histogram, error) {
	values := make([]float64, len(games))
	for idx, game := range games {
		values[idx] = float64(game.PlayerScore)
	}
	return BuildHistogram(values, HistogramOptions{Buckets: buckets})
}

// DurationHistogram builds a histogram of game durations in seconds.
func DurationHistogram(games []Game, buckets int) (Histogram, error) {
	values := make([]float64, len(games))
	for idx, game := range games {
		values[idx] = game.Duration().Seconds()
	}
	return BuildHistogram(values, HistogramOptions{Buckets: buckets})
}

// Mean returns the mean of the slice (0 if empty).
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, val := range values {
		sum += val
	}
	return sum / float64(len(values))
}

// Median returns the median (copies and sorts the slice).
func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := make([]float64, len(values))
	copy(cp, values)
	sort.Float64s(cp)
	length := len(cp)
	if length%2 == 1 {
		return cp[length/2]
	}
	return (cp[length/2-1] + cp[length/2]) / 2.0
}

// Percentile computes the value at percentile p (0..100).
func Percentile(values []float64, pct float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if pct <= 0 {
		return values[0]
	}
	if pct >= 100 {
		return values[len(values)-1]
	}
	cp := make([]float64, len(values))
	copy(cp, values)
	sort.Float64s(cp)
	rank := (pct / 100.0) * float64(len(cp)-1)
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
	minVal, maxVal := minMax(values)
	return Summary{
		N:      len(values),
		Mean:   Mean(values),
		Median: Median(values),
		Min:    minVal,
		Max:    maxVal,
		StdDev: StdDev(values),
		P25:    Percentile(values, 25),
		P75:    Percentile(values, 75),
		P95:    Percentile(values, 95),
		P99:    Percentile(values, 99),
	}
}
