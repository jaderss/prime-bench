package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/jaderss/prime-bench/apps/go/metrics"
)

const (
	accounts = 10000
	csvBegin = "----- BEGIN CSV -----"
	csvEnd   = "----- END CSV -----"
)

type config struct {
	target     string
	users      int
	duration   time.Duration
	rampUp     time.Duration
	thinkTime  time.Duration
	timeout    time.Duration
	bits       int
	scenario   string
	mockURL    string
	modePrefix string
	csvStdout  bool
	runID      string
	resultsDir string
}

type targetStats struct {
	Lang       string  `json:"lang"`
	Version    string  `json:"version"`
	CPUs       int     `json:"cpus"`
	RssMB      float64 `json:"rss_mb"`
	CPUSeconds float64 `json:"cpu_seconds"`
}

func loadConfig() config {
	return config{
		target:     cmp.Or(os.Getenv("TARGET"), "http://localhost:8080"),
		users:      envInt("USERS", 1000),
		duration:   envDuration("DURATION", 60*time.Second),
		rampUp:     envDuration("RAMP_UP", 10*time.Second),
		thinkTime:  envDuration("THINK_TIME", 500*time.Millisecond),
		timeout:    envDuration("TIMEOUT", 30*time.Second),
		bits:       envInt("BITS", 1024),
		scenario:   cmp.Or(os.Getenv("SCENARIO"), "prime"),
		mockURL:    os.Getenv("MOCK_URL"),
		modePrefix: os.Getenv("MODE_PREFIX"),
		csvStdout:  os.Getenv("CSV_STDOUT") == "true",
		runID:      cmp.Or(os.Getenv("RUN_ID"), "local"),
		resultsDir: cmp.Or(os.Getenv("RESULTS_DIR"), "results"),
	}
}

func envInt(key string, fallback int) int {
	if value, err := strconv.Atoi(os.Getenv(key)); err == nil && value > 0 {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if value, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return value
	}
	return fallback
}

func main() {
	if err := run(loadConfig()); err != nil {
		log.Fatal(err)
	}
}

func run(cfg config) error {
	client := &http.Client{Transport: &http.Transport{
		MaxIdleConns:        cfg.users,
		MaxIdleConnsPerHost: cfg.users,
		IdleConnTimeout:     90 * time.Second,
	}}
	target, err := waitForTarget(client, cfg.target)
	if err != nil {
		return err
	}
	nextURL, mode, bits, err := scenario(cfg)
	if err != nil {
		return err
	}
	mockBefore, _ := fetchStats(client, cfg.mockURL)
	rec := &metrics.Recorder{}
	info := metrics.RunInfo{
		Lang: target.Lang, LangVersion: target.Version, Mode: cfg.modePrefix + mode, RunID: cfg.runID,
		CPUs: target.CPUs, Bits: bits, Concurrency: cfg.users,
	}
	sampler, err := metrics.StartSampler(cfg.resultsDir, info, rec, remoteStats(client, cfg.target))
	if err != nil {
		return err
	}
	log.Printf("%d users, %s scenario, against %s %s at %s for %s", cfg.users, mode, target.Lang, target.Version, cfg.target, cfg.duration)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.duration)
	defer cancel()
	start := time.Now()
	var users sync.WaitGroup
	for i := range cfg.users {
		delay := cfg.rampUp * time.Duration(i) / time.Duration(cfg.users)
		users.Go(func() { user(ctx, client, cfg, nextURL, delay, rec) })
	}
	users.Wait()
	summary, err := sampler.Stop()
	if err == nil && cfg.csvStdout {
		err = printCSV(cfg, info)
	}
	log.Printf("%s %s %s users=%d %v", target.Lang, target.Version, mode, cfg.users, summary)
	if mockAfter, mockErr := fetchStats(client, cfg.mockURL); cfg.mockURL != "" && mockErr == nil {
		cpu := (mockAfter.CPUSeconds - mockBefore.CPUSeconds) / time.Since(start).Seconds() * 100
		log.Printf("mock downstream cpu=%.0f%% rss=%.1fMB", cpu, mockAfter.RssMB)
	}
	return err
}

func scenario(cfg config) (func() string, string, int, error) {
	switch cfg.scenario {
	case "prime":
		url := fmt.Sprintf("%s/prime?bits=%d", cfg.target, cfg.bits)
		return func() string { return url }, "server", cfg.bits, nil
	case "gateway":
		return func() string {
			return fmt.Sprintf("%s/aggregate?account=acc-%d", cfg.target, rand.N(accounts))
		}, "gateway", 0, nil
	default:
		return nil, "", 0, fmt.Errorf("unknown SCENARIO %q, expected prime or gateway", cfg.scenario)
	}
}

func user(ctx context.Context, client *http.Client, cfg config, nextURL func() string, delay time.Duration, rec *metrics.Recorder) {
	if !sleep(ctx, delay) {
		return
	}
	for ctx.Err() == nil {
		start := time.Now()
		err := call(ctx, client, nextURL(), cfg.timeout)
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			rec.Fail()
		default:
			rec.Record(time.Since(start))
		}
		if !sleep(ctx, jitter(cfg.thinkTime)) {
			return
		}
	}
}

func call(ctx context.Context, client *http.Client, url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func jitter(d time.Duration) time.Duration {
	return d/2 + rand.N(d+1)
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func waitForTarget(client *http.Client, target string) (targetStats, error) {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if stats, err := fetchStats(client, target); err == nil {
			return stats, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return targetStats{}, errors.New("target did not answer /stats within 60s: " + target)
}

func fetchStats(client *http.Client, target string) (targetStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target+"/stats", nil)
	if err != nil {
		return targetStats{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return targetStats{}, err
	}
	defer resp.Body.Close()
	var stats targetStats
	return stats, json.NewDecoder(resp.Body).Decode(&stats)
}

func remoteStats(client *http.Client, target string) metrics.StatsSource {
	var last metrics.ProcessStats
	return func() metrics.ProcessStats {
		if stats, err := fetchStats(client, target); err == nil {
			last = metrics.ProcessStats{RssMB: stats.RssMB, CPUSeconds: stats.CPUSeconds}
		}
		return last
	}
}

func printCSV(cfg config, info metrics.RunInfo) error {
	body, err := os.ReadFile(filepath.Join(cfg.resultsDir, fmt.Sprintf("%s_%s_%s.csv", info.RunID, info.Mode, info.Lang)))
	if err != nil {
		return err
	}
	fmt.Printf("%s\n%s%s\n", csvBegin, body, csvEnd)
	return nil
}
