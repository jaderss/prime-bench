package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/jaderss/prime-bench/apps/go/metrics"
)

const minBits, maxBits = 64, 8192

func serve(cfg config) error {
	log.Printf("go %s listening on :%s", langVersion(), cfg.port)
	server := &http.Server{Addr: ":" + cfg.port, Handler: newHandler(cfg), ReadHeaderTimeout: 10 * time.Second}
	return server.ListenAndServe()
}

func newHandler(cfg config) http.Handler {
	generators := sync.Pool{New: func() any { return newGenerator(cfg.rounds) }}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /prime", func(w http.ResponseWriter, r *http.Request) {
		bits, err := parseBits(r.URL.Query(), cfg.bits)
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
	downstream := newGateway(cfg.downstream)
	mux.HandleFunc("GET /aggregate", func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account")
		if !validAccountID(accountID) {
			http.Error(w, "account must be 1 to 64 letters, digits or dashes", http.StatusBadRequest)
			return
		}
		result, err := downstream.aggregate(r.Context(), accountID)
		if err != nil {
			log.Printf("downstream call failed: %v", err)
			http.Error(w, "downstream unavailable", http.StatusBadGateway)
			return
		}
		writeJSON(w, result)
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
	return mux
}

func parseBits(query url.Values, fallback int) (int, error) {
	if !query.Has("bits") {
		return fallback, nil
	}
	bits, err := strconv.Atoi(query.Get("bits"))
	if err != nil || bits < minBits || bits > maxBits {
		return 0, fmt.Errorf("bits must be an integer between %d and %d", minBits, maxBits)
	}
	return bits, nil
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(body)
}
