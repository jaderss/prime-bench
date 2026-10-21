package metrics

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var header = []string{
	"kind", "lang", "lang_version", "mode", "run_id", "elapsed_s", "cpus", "bits", "concurrency",
	"completed", "errors", "throughput_ops_s", "latency_p50_ms", "latency_p95_ms", "latency_p99_ms",
	"latency_max_ms", "rss_mb", "cpu_pct",
}

const minInterval = 250 * time.Millisecond

type ProcessStats struct {
	RssMB      float64
	CPUSeconds float64
}

type StatsSource func() ProcessStats

func ReadProcessStats() ProcessStats {
	return ProcessStats{RssMB: readRSSMB(), CPUSeconds: readCPUSeconds()}
}

func readRSSMB() float64 {
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for line := range strings.Lines(string(status)) {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "VmRSS:" {
			kb, _ := strconv.ParseFloat(fields[1], 64)
			return kb / 1024
		}
	}
	return 0
}

func readCPUSeconds() float64 {
	stat, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	utime, _ := strconv.ParseFloat(fields[11], 64)
	stime, _ := strconv.ParseFloat(fields[12], 64)
	return (utime + stime) / 100
}

type Recorder struct {
	mu             sync.Mutex
	interval, all  []float64
	intervalErrors int
	allErrors      int
}

func (r *Recorder) Record(latency time.Duration) {
	ms := float64(latency.Nanoseconds()) / 1e6
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interval = append(r.interval, ms)
	r.all = append(r.all, ms)
}

func (r *Recorder) Fail() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.intervalErrors++
	r.allErrors++
}

func (r *Recorder) drain() ([]float64, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	latencies, errs := r.interval, r.intervalErrors
	r.interval, r.intervalErrors = nil, 0
	return latencies, errs
}

func (r *Recorder) total() ([]float64, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.all), r.allErrors
}

type RunInfo struct {
	Lang, LangVersion, Mode, RunID string
	CPUs, Bits, Concurrency        int
}

type Row struct {
	kind       string
	elapsed    float64
	sorted     []float64
	errors     int
	wall       float64
	rssMB      float64
	cpuPercent float64
}

func newRow(kind string, elapsed float64, latencies []float64, errors int, wall, rssMB, cpuPercent float64) Row {
	return Row{kind, elapsed, slices.Sorted(slices.Values(latencies)), errors, wall, rssMB, cpuPercent}
}

func (r Row) throughput() float64 {
	return float64(len(r.sorted)) / r.wall
}

func (r Row) String() string {
	return fmt.Sprintf("completed=%d errors=%d throughput=%.2f/s p50=%.2fms p99=%.2fms peak_rss=%.1fMB cpu=%.0f%%",
		len(r.sorted), r.errors, r.throughput(), Percentile(r.sorted, 50), Percentile(r.sorted, 99), r.rssMB, r.cpuPercent)
}

type Sampler struct {
	info     RunInfo
	rec      *Recorder
	stats    StatsSource
	writer   *csv.Writer
	file     *os.File
	start    time.Time
	startCPU float64
	last     time.Time
	lastCPU  float64
	peakRSS  float64
	done     chan struct{}
	ticking  sync.WaitGroup
}

func StartSampler(dir string, info RunInfo, rec *Recorder, stats StatsSource) (*Sampler, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	file, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s_%s_%s.csv", info.RunID, info.Mode, info.Lang)))
	if err != nil {
		return nil, err
	}
	now, cpu := time.Now(), stats().CPUSeconds
	s := &Sampler{
		info: info, rec: rec, stats: stats, writer: csv.NewWriter(file), file: file,
		start: now, startCPU: cpu, last: now, lastCPU: cpu, done: make(chan struct{}),
	}
	s.writer.Write(header)
	s.ticking.Go(s.loop)
	return s, nil
}

func (s *Sampler) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.sample()
		case <-s.done:
			return
		}
	}
}

func (s *Sampler) sample() {
	now, stats := time.Now(), s.stats()
	latencies, errs := s.rec.drain()
	wall := now.Sub(s.last).Seconds()
	s.peakRSS = max(s.peakRSS, stats.RssMB)
	s.write(newRow("interval", now.Sub(s.start).Seconds(), latencies, errs, wall, stats.RssMB, (stats.CPUSeconds-s.lastCPU)/wall*100))
	s.last, s.lastCPU = now, stats.CPUSeconds
}

func (s *Sampler) Stop() (Row, error) {
	close(s.done)
	s.ticking.Wait()
	if time.Since(s.last) >= minInterval {
		s.sample()
	}
	now, stats := time.Now(), s.stats()
	all, errs := s.rec.total()
	wall := now.Sub(s.start).Seconds()
	peak := max(s.peakRSS, stats.RssMB)
	summary := newRow("summary", wall, all, errs, wall, peak, (stats.CPUSeconds-s.startCPU)/wall*100)
	s.write(summary)
	if err := s.writer.Error(); err != nil {
		return summary, err
	}
	return summary, s.file.Close()
}

func (s *Sampler) write(r Row) {
	s.writer.Write([]string{
		r.kind, s.info.Lang, s.info.LangVersion, s.info.Mode, s.info.RunID, format(r.elapsed),
		strconv.Itoa(s.info.CPUs), strconv.Itoa(s.info.Bits), strconv.Itoa(s.info.Concurrency),
		strconv.Itoa(len(r.sorted)), strconv.Itoa(r.errors), format(r.throughput()),
		format(Percentile(r.sorted, 50)), format(Percentile(r.sorted, 95)), format(Percentile(r.sorted, 99)),
		format(Percentile(r.sorted, 100)), format(r.rssMB), format(r.cpuPercent),
	})
	s.writer.Flush()
}

func Percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	return sorted[max(rank, 1)-1]
}

func format(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}
