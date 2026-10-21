package main

import (
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jaderss/prime-bench/apps/go/metrics"
)

func runBatch(cfg config) error {
	rec := &metrics.Recorder{}
	info := metrics.RunInfo{Lang: "go", LangVersion: langVersion(), Mode: "batch", RunID: cfg.runID, CPUs: runtime.GOMAXPROCS(0), Bits: cfg.bits, Concurrency: cfg.concurrency}
	s, err := metrics.StartSampler(cfg.resultsDir, info, rec, metrics.ReadProcessStats)
	if err != nil {
		return err
	}
	remaining := atomic.Int64{}
	remaining.Store(int64(cfg.count))
	var workers sync.WaitGroup
	for range cfg.concurrency {
		workers.Go(func() {
			g := newGenerator(cfg.rounds)
			for remaining.Add(-1) >= 0 {
				start := time.Now()
				g.probablePrime(cfg.bits)
				rec.Record(time.Since(start))
			}
		})
	}
	workers.Wait()
	summary, err := s.Stop()
	log.Printf("go %s batch bits=%d count=%d workers=%d %v", info.LangVersion, cfg.bits, cfg.count, cfg.concurrency, summary)
	return err
}
