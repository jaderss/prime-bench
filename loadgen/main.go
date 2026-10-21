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
	"strconv"
	"sync"
	"time"

	"github.com/jaderss/prime-bench/apps/go/metrics"
)

type config struct {
	target     string
	users      int
	duration   time.Duration
	rampUp     time.Duration
	thinkTime  time.Duration
	timeout    time.Duration
	bits       int
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
	rec := &metrics.Recorder{}
	info := metrics.RunInfo{
		Lang: target.Lang, LangVersion: target.Version, Mode: "server", RunID: cfg.runID,
		CPUs: target.CPUs, Bits: cfg.bits, Concurrency: cfg.users,
	}
	sampler, err := metrics.StartSampler(cfg.resultsDir, info, rec, remoteStats(client, cfg.target))
	if err != nil {
		return err
	}
	log.Printf("%d users against %s %s at %s for %s", cfg.users, target.Lang, target.Version, cfg.target, cfg.duration)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.duration)
	defer cancel()
	url := fmt.Sprintf("%s/prime?bits=%d", cfg.target, cfg.bits)
	var users sync.WaitGroup
	for i := range cfg.users {
		delay := cfg.rampUp * time.Duration(i) / time.Duration(cfg.users)
		users.Go(func() { user(ctx, client, cfg, url, delay, rec) })
	}
	users.Wait()
	summary, err := sampler.Stop()
	log.Printf("%s %s server users=%d %v", target.Lang, target.Version, cfg.users, summary)
	return err
}

func user(ctx context.Context, client *http.Client, cfg config, url string, delay time.Duration, rec *metrics.Recorder) {
	if !sleep(ctx, delay) {
		return
	}
	for ctx.Err() == nil {
		start := time.Now()
		err := call(ctx, client, url, cfg.timeout)
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
