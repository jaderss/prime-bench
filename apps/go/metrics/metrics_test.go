package metrics

import (
	"math"
	"testing"
)

func TestPercentileUsesNearestRank(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	for p, want := range map[float64]float64{50: 5, 95: 10, 99: 10, 100: 10, 10: 1} {
		if got := Percentile(sorted, p); got != want {
			t.Errorf("p%v = %v, want %v", p, got, want)
		}
	}
	if Percentile(nil, 50) != 0 {
		t.Error("empty input should give 0")
	}
}

const (
	sampleStat   = "4242 (my (odd) proc) S 1 4242 4242 0 -1 4194560 2000 0 0 0 250 50 0 0 20 0 9 0 1000 123456 789 0\n"
	sampleStatus = "Name:\tjava\nVmPeak:\t  999999 kB\nVmRSS:\t  204800 kB\nThreads:\t9\n"
)

func TestCPUSecondsAddsUserAndSystemTicksAfterTheLastParenthesis(t *testing.T) {
	if got := parseCPUSeconds(sampleStat); math.Abs(got-3) > 1e-9 {
		t.Errorf("got %v, want 3", got)
	}
}

func TestRSSReadsVmRSSInMegabytes(t *testing.T) {
	if got := parseRSSMB(sampleStatus); math.Abs(got-200) > 1e-9 {
		t.Errorf("got %v, want 200", got)
	}
	if got := parseRSSMB("Name:\tkthreadd\n"); got != 0 {
		t.Errorf("got %v, want 0 without VmRSS", got)
	}
}
