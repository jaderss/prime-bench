package metrics

import "testing"

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
