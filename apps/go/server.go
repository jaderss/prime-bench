package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/jaderss/prime-bench/apps/go/metrics"
)

const minBits, maxBits = 64, 8192

func serve(cfg config) error {
	generators := sync.Pool{New: func() any { return newGenerator(cfg.rounds) }}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /prime", func(w http.ResponseWriter, r *http.Request) {
		bits, err := parseBits(r.URL.Query().Get("bits"), cfg.bits)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		g := generators.Get().(*generator)
		defer generators.Put(g)
		start := time.Now()
		prime := g.probablePrime(bits)
		writeJSON(w, map[string]any{"bits": bits, "prime": prime.String(), "ms": float64(time.Since(start).Microseconds()) / 1000})
	})
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		stats := metrics.ReadProcessStats()
		writeJSON(w, map[string]any{
			"lang": "go", "version": langVersion(), "cpus": runtime.GOMAXPROCS(0),
			"rss_mb": stats.RssMB, "cpu_seconds": stats.CPUSeconds,
		})
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	log.Printf("go %s listening on :%s", langVersion(), cfg.port)
	return http.ListenAndServe(":"+cfg.port, mux)
}

func parseBits(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	bits, err := strconv.Atoi(raw)
	if err != nil || bits < minBits || bits > maxBits {
		return 0, fmt.Errorf("bits must be an integer between %d and %d", minBits, maxBits)
	}
	return bits, nil
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(body)
}
